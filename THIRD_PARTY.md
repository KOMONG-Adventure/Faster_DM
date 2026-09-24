# YouTube таталтын туслах хэрэгслүүд

## Dashboard ба Windows tray

- go-toast/v2 2.0.3 — Unlicense OR MIT, https://git.sr.ht/~jackmordaunt/go-toast
  Windows мэдэгдэл. Лиценз: `licenses/go-toast-LICENSE.txt`.
- Driver.js 1.3.6 — MIT, https://github.com/kamranahmedse/driver.js
  Лиценз: `licenses/driverjs-LICENSE.txt`.
- energye/systray 1.0.3 — Apache-2.0, https://github.com/energye/systray
  Лиценз: `licenses/systray-LICENSE.txt`.

## Media хэрэгслүүд

Faster DM нь эдгээр тусдаа програмыг командын аргументаар ажиллуулна.
Энгийн HTTP таталт Go engine-ээр хийгдэх бөгөөд эдгээр хэрэгсэл шаардлагагүй.

- yt-dlp 2026.08.19: https://github.com/yt-dlp/yt-dlp/releases/tag/2026.08.19
  Эх код: https://github.com/yt-dlp/yt-dlp/tree/2026.08.19
  Төслийн лиценз Unlicense; Windows executable-д GPLv3+ зэрэг гуравдагч багцууд ордог.
  https://github.com/yt-dlp/yt-dlp#license
- Deno 2.9.7: https://github.com/denoland/deno/releases/tag/v2.9.7
  Эх код ба MIT лиценз: https://github.com/denoland/deno/tree/v2.9.7
- FFmpeg 9.0.2 Windows essentials build: https://github.com/GyanD/codexffmpeg/releases/tag/9.0.2
  FFmpeg эх код: https://ffmpeg.org/download.html
  Build-ийн лиценз болон configure мэдээлэл нь тухайн түгээлтэд багтана.
  https://ffmpeg.org/legal.html

`scripts/setup-media.ps1` нь executable/archive бүрийг SHA-256-аар шалгаж байж суулгана.
Бүх хувилбарын URL болон checksum түгжигдсэн. Шинэ хувилбарын artifact-ийг
шалгаж байж script-ийн URL, hash-ийг шинэчилнэ.
Хэрэгслүүдийг дахин түгээхдээ тухайн upstream түгээлтийн лиценз, эх кодын шаардлагыг дагана.
