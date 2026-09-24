import { Archive, File, FileText, Film, Image, Music2 } from 'lucide-react';

export const categories = [
  { key: 'Videos', label: 'Видео', icon: Film, className: 'violet', extensions: 'mp4 mkv mov avi webm m4v wmv flv mpeg mpg ts' },
  { key: 'Photos', label: 'Зураг', icon: Image, className: 'rose', extensions: 'jpg jpeg png gif webp svg avif heic bmp tif tiff ico raw' },
  { key: 'Archives', label: 'Архив', icon: Archive, className: 'amber', extensions: 'zip rar 7z gz bz2 xz tar tgz zst iso' },
  { key: 'Audio', label: 'Аудио', icon: Music2, className: 'blue', extensions: 'mp3 wav flac aac ogg m4a opus wma aiff' },
  { key: 'Documents', label: 'Баримт', icon: FileText, className: 'teal', extensions: 'pdf doc docx xls xlsx ppt pptx txt csv md epub rtf odt json' },
  { key: 'Other', label: 'Бусад', icon: File, className: 'slate', extensions: '' },
];
export const categoryFor = (name: string) => categories.find(c => c.extensions.split(' ').includes(name.split('.').pop()?.toLowerCase() || '!')) ?? categories[5];
export const categoryByKey = (key: string) => categories.find(c => c.key === key) ?? categories[5];
export function suggestName(value: string): string {
  try {
    const url = new URL(value);
    const name = decodeURIComponent(url.pathname.split('/').filter(Boolean).pop() || 'download.bin');
    return name.replace(/[<>:"/\\|?*\x00-\x1f]/g, '_').replace(/[. ]+$/, '').slice(0, 150) || 'download.bin';
  } catch { return ''; }
}
