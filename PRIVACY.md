# RadLog privacy policy

*Last updated: 2026-11-13* · [Português](PRIVACY.pt-BR.md)

RadLog is a free, open-source desktop app that records readings from personal radiation meters (GQ GMC-800 and Radiacode 102). This page explains what happens to your data.

## Short version

- RadLog has **no server** of its own. There is no account with us, no analytics, no ads and no tracking.
- Your readings stay on your computer, in the app's data folder.
- If you turn on the optional cloud backup, RadLog stores an **encrypted** copy of your data in **your own Google Drive**. Nobody else, including the RadLog developers and Google, can read it.

## Optional cloud backup (Google Drive)

The cloud backup is off by default. When you turn it on and sign in with Google:

- **Scopes requested:** `openid`, `email` and `https://www.googleapis.com/auth/drive.file`.
  - `drive.file` only lets RadLog see and change the files and the folder that RadLog itself created in your Drive. It cannot see, read or change any other file in your Drive.
  - `email` is used only to show which Google account is connected, on the app's settings screen. It is not written to logs, backups, PDF reports or the cloud.
- **What is uploaded:** your readings, alerts, notes, measurements and their attachments, encrypted on your computer with AES-256-GCM before upload. The key comes from a recovery code that only you have. File names in Drive are random-looking, so they do not reveal dates or devices.
- **What is never uploaded:** your location, coordinates or photo metadata (photo location is stripped before upload).
- **Where it goes:** directly from your computer to Google Drive. It does not pass through any server of ours.
- **Tokens:** the Google sign-in token and your recovery code are stored only on your computer (protected by Windows DPAPI on Windows).
- **Turning it off:** "Disconnect from Google" in the app removes the token and code from your computer. You can also revoke access at https://myaccount.google.com/permissions and delete the RadLog folder from your Drive at any time.

RadLog's use and transfer of information received from Google APIs adheres to the [Google API Services User Data Policy](https://developers.google.com/terms/api-services-user-data-policy), including the Limited Use requirements. Data from Google APIs is used only to provide the backup feature to you. It is not sold, not shared with third parties, not used for advertising and not used to train AI models.

## Other network connections

RadLog only connects to the internet for these optional features:

- **Update check:** GitHub (`api.github.com`) to see if a new version exists.
- **Weather and rain radar:** Open-Meteo and RainViewer, using the place you choose in settings. Those services receive the coordinates needed to answer the request.
- **Sync between your PCs on your local network:** direct, encrypted connection between your own computers.

## Data sharing

We do not collect, sell or share any personal data. We have no access to your data.

## Contact

Questions or requests: open an issue at https://github.com/matheusAFreitas/radlog/issues.
