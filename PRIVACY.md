# RadLog privacy policy

*Last updated: 2026-11-14* · [Português](PRIVACY.pt-BR.md)

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
- **What is uploaded:** everything is encrypted on your computer with AES-256-GCM before upload. The key comes from a recovery code that only you have. File names in Drive are random-looking, so they do not reveal dates or devices. Uploaded:
  - readings (GMC-800 minutes and Radiacode 102 rows) and spectra;
  - alerts, notes and measurements, with their attachments;
  - times away from home, with the name you gave each one (if that name is a place, it is uploaded, encrypted);
  - the settings and calibration histories of the meters;
  - this device's name in the cloud, which is the PC name by default (you can change it in the app).
- **What is never uploaded:** the place and coordinates from the settings, and photo metadata (location is stripped from jpg, png, webp, heic and avif photos before upload). Attachments of types that can carry a location and that the app cannot clean (videos, TIFF and RAW photos) stay only on your computer: they do not go to the cloud, the backup or the local network sync.
- **Where it goes:** directly from your computer to Google Drive. It does not pass through any server of ours.
- **Tokens:** the Google sign-in token and your recovery code are stored only on your computer. On Windows they are protected by DPAPI; on Linux they are encrypted with a key kept in the system keyring (GNOME Keyring or KWallet) and, without a keyring, kept in a file only your user can read. On Windows, the code copied in the app stays out of the clipboard history and is cleared from the clipboard after 1 minute.
- **Turning it off:** "Disconnect from Google" in the app removes the token and code from your computer. If it is the only device in the cloud, RadLog is also removed from your Google account's app list. You can also revoke access at https://myaccount.google.com/permissions and delete the RadLog folder from your Drive at any time.

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
