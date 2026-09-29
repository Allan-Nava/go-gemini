# Security policy

## Supported versions

Only the latest 1.x release gets security fixes; they ship as a patch or minor release. Upgrading within
1.x never breaks your code.

## Reporting a vulnerability

Please report it privately through GitHub:
[Security → Report a vulnerability](https://github.com/Allan-Nava/go-gemini/security/advisories/new).
Do not open a public issue, and do not include a real API key in the report.

You can expect an acknowledgement within a few days. Once a fix is released, the advisory is
published with credit to you unless you prefer otherwise.

## What the SDK does with your API key

- It reads the key from `WithAPIKey` or the `GEMINI_API_KEY` environment variable and keeps it in memory.
- It sends the key only in the `x-goog-api-key` header, never in the URL, and never includes it in
  error messages.
- It sends the key only over HTTPS; plain HTTP is accepted for loopback test servers.
- It refuses redirects to another host or scheme (`ErrRedirectOtherHost`), because `net/http` would
  otherwise copy the header to the redirect target. With `WithHTTPClient`, a client without
  `CheckRedirect` gets the same rule; if you set your own `CheckRedirect`, that responsibility is yours.

If a key leaks (a commit, a log, a screenshot, a chat), delete it in
[Google AI Studio](https://aistudio.google.com/apikey) and create a new one: removing it from the place
where it leaked is not enough.
