# Faster DM privacy policy

Last updated: 2026-09-26

Faster DM is a desktop application. Its application code does not implement
advertising, usage analytics, or automatic upload of download history to the
project maintainers. No Faster DM account is required to use the app.

## Data stored on your computer

- Download history contains file names, paths, source URLs, progress, timestamps,
  and error details. On Windows it is stored in `%APPDATA%\FasterDM\history`.
- Partial downloads and resume metadata are stored beside the destination file.
  HTTP resume metadata can contain the source URL and server ETag.
- Settings and removed-history markers are kept locally. Removing a history row
  retains a local marker, including its file path, to prevent automatic re-import;
  its source URL is cleared. Downloaded files are not deleted by this action.
- Clipboard-autofill and onboarding preferences are kept in local WebView storage.
- Update installers, installer logs, and helper-tool files may remain in local
  application/cache folders. Uninstalling does not automatically erase downloaded
  files or history. Source URLs may contain access tokens; protect local records.

## Clipboard

Clipboard autofill is enabled by default. While the app interface is visible and
the feature is enabled, the app checks for HTTP/HTTPS URLs and fills the input.
Ordinary clipboard text is not returned to the interface. Copying a URL alone
does not start a download or send it to the project's maintainers. You can disable
this behavior with the clipboard-autofill checkbox.

## Network requests and third parties

- Checking disk requirements or starting a download contacts the supplied URL, its redirects, and servers needed
  to retrieve the requested content. These servers receive normal connection
  information such as your IP address and request headers. Their policies apply.
- YouTube downloads use yt-dlp and associated tools to contact YouTube and its
  content delivery services. See [Google's privacy policy](https://policies.google.com/privacy)
  and [yt-dlp documentation](https://github.com/yt-dlp/yt-dlp).
- Checking/installing updates contacts GitHub and its asset delivery services.
  Setup scripts can download dependencies from their documented upstream sources.
  See [GitHub's privacy statement](https://docs.github.com/en/site-policy/privacy-policies/github-general-privacy-statement)
  and [third-party notices](THIRD_PARTY.md).
- Windows, WebView2, and third-party tools may have their own network activity and
  privacy controls. See [Microsoft's privacy statement](https://privacy.microsoft.com/privacystatement).

Download completion notifications may display file names on your Windows desktop.
Notifications can be disabled in Faster DM's download settings.

## Contact

Privacy questions can be raised through the
[project's GitHub issues](https://github.com/KOMONG-Adventure/Faster_DM/issues).
Do not post private download URLs, tokens, or personal files in public issues.

## Optional browser extension

The Chrome/Edge extension sends only the HTTP/HTTPS URL you select to a local
native host. Pending URLs are stored in `%APPDATA%\FasterDM\browser-inbox`
until the app accepts them. The popup reads the active tab URL and, only with
your permission, displays ten recent browser downloads. It does not forward
cookies, passwords, or browsing history to maintainers. Local extension storage
keeps the last delivery status. Browser downloads are not automatically canceled.
