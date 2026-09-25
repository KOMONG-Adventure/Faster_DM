# Code signing policy

## Current status

Faster DM is applying to the [SignPath Foundation](https://signpath.org/) for
open-source code signing. The application is pending preparation/review; no
approval, certificate, or SignPath-signed release is claimed. Existing releases
are unsigned unless explicitly identified and verified otherwise.

If accepted, the intended attribution is: "Free code signing provided by
[SignPath.io](https://signpath.io/), certificate by
[SignPath Foundation](https://signpath.org/)."

## Responsibilities and release process

- Project owner, committer, reviewer, and proposed signing approver:
  [KOMONG-Adventure](https://github.com/KOMONG-Adventure).
- Signing participants must enable multi-factor authentication on GitHub and
  SignPath before participating in signing.
- External contributions must be reviewed by the project maintainer.
- Signing requests must originate from verifiable repository builds and receive
  explicit maintainer approval. Signing automation is not configured yet.
- The existing build workflow is
  [Windows installer release](.github/workflows/release.yml).
- Intended signing targets are our own FasterDM executable and Windows installer.
  Upstream binaries such as yt-dlp, Deno, and FFmpeg retain their upstream licenses
  and signing status; they will not be re-signed with a Foundation project
  certificate. Their presence in a signed installer does not imply they are signed.
- Signatures and timestamps must be verified before signed releases are published.

Faster DM's own code is available under the [MIT license](LICENSE).
See [third-party notices](THIRD_PARTY.md) for separately licensed components.

## Privacy

See the [privacy policy](PRIVACY.md) for local history, clipboard access,
user-requested network connections, and third-party services.

## Downloads

Use the [official download page](DISTRIBUTION.md). A valid signature authenticates
the publisher and integrity of a file; it does not guarantee that every Windows
security product will allow it.
