# When the CDN refuses to serve media

Read this if `whatsapp download` fails with a 403, or if a batch of downloads
reports success and leaves fewer files than you asked for. Both happened at
once on 3 October 2026, and they were two separate bugs.

## What you would have seen

```
whatsapp download ACB4E8295D205BAFB1C21E3D7B0BF779 --chat "31634727863-1474609951@g.us"
# => {"error":"download failed: download failed with status code 403"}
```

Every media download through this tool failed this way, for every chat and
every media type, including files that had downloaded fine weeks earlier.

## Bug one: the direct path was stripped of its query string

WhatsApp signs each media path with `oh=` and `oe=` parameters, and the CDN
answers 403 without them. `extractDirectPathFromURL` threw them away:

```go
p := strings.SplitN(parts[1], "?", 2)[0]   // the defect
return "/" + p
```

Two things then go wrong. The signature is gone, and whatsmeow appends its own
parameters with `&` rather than `?`, because it assumes the direct path already
carries a query string. Look at `DownloadMediaWithPath` in whatsmeow's
`download.go`:

```go
mediaURL := fmt.Sprintf("https://%s%s&hash=%s&mms-type=%s&__wa-mms=", host.Hostname, directPath, ...)
```

With the query stripped there is no `?` anywhere in the result, so `&hash=...`
lands inside the path:

```
https://mmg.whatsapp.net/v/t62.7117-24/832665140_..._n.enc&hash=We0G...&mms-type=audio&__wa-mms=
```

Measured against the live CDN on the same object, same minute:

| Direct path | Response |
|---|---|
| Query stripped, as the tool built it | **403**, 12 bytes |
| Query preserved | **200**, 729130 bytes |

**This was a latent bug, not a regression.** 95 voice notes had downloaded
successfully before, and a September file that worked then returns 403 now when
asked for the same way. Nothing in this repo changed. WhatsApp began enforcing
the signature, and a URL that had been tolerated stopped being served. The code
had been wrong the whole time and the CDN had been covering for it.

`direct_path` is also now persisted from the message rather than re-derived from
the URL. It is the authoritative locator, and WhatsApp leaves `url` empty on
some messages: 26 image and video rows in a 34,000-message store have no URL at
all, and used to fail with `incomplete media info`.

## Bug two: colliding filenames overwrote each other, silently

Stored filenames are stamped with the sync clock rather than the message:

```go
fmt.Sprintf("audio_%s.ogg", time.Now().Format("20060102_150405"))
```

A history sync writes thousands of rows per second, so everything it names in
the same second gets the same name. Ten voice notes received overnight shared
four filenames. Three distinct messages were all called
`audio_20261003_150932.ogg`.

Fixing the 403 alone would therefore have produced four files from ten
downloads, each overwriting the last, **with all ten reporting success**. That
is the worst way to lose a file: you are told it arrived, and the only evidence
it did not is a directory listing nobody checks.

`uniqueMediaFilename` resolves this at the one place output paths are created,
so it covers rows already in the store as well as new ones. It appends an
eight-character message-ID discriminator when more than one message in the chat
claims a name, and leaves uncontested names alone:

```
audio_20261003_150932_ACB4E829.ogg
audio_20261003_150932_AC50A201.ogg
audio_20261003_150932_AC7E2FFC.ogg
audio_20261003_150952.ogg            # only one message claims this
```

If the store cannot be read to count claimants, the discriminator is added
anyway. An ugly name costs nothing; an overwritten file cannot be recovered.

Across the store, 3417 media messages shared a filename with a sibling in the
same chat.

## The trap if you are changing this code

**A direct path is a path *and* its query string.** Anything that parses one,
rewrites a host, or normalises a URL has to carry the query through, or the
object stops being served. Treat the whole string as opaque.

**Do not test this by comparing the derived path against a literal.** That is
the check that would have stayed green for the entire outage, because the
stripped path is a perfectly reasonable looking path. `mediapath_test.go`
instead composes the URL the way whatsmeow composes it, parses the result, and
asserts that `oh`, `oe`, `hash` and `mms-type` all land in the query rather than
the path. Revert `extractDirectPathFromURL` to the stripping version and that
test names the mechanism in its failure output.

## Checking a download actually worked

An exit status of 0 is not enough, and neither is a file existing at the path.
Compare the bytes on disk against what the message declared:

```bash
sqlite3 -readonly ~/.config/whatsapp-cli/store/messages.db \
  "SELECT file_length FROM messages WHERE id = '<msg-id>'"
stat -f%z "<downloaded path>"
file -b "<downloaded path>"      # expect: Ogg data, Opus audio
```

whatsmeow verifies the SHA256 itself and fails with `ErrInvalidMediaSHA256`
rather than writing a corrupt file, so a success plus a matching length plus a
recognised container is enough.
