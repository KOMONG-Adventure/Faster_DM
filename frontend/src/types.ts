export type Status = 'queued' | 'probing' | 'downloading' | 'paused' | 'canceling' | 'complete' | 'failed' | 'canceled';
export interface Chunk { id: number; start: number; end: number; downloaded: number; bytesPerSecond: number; status: string; retries: number }
export interface Progress { networkBytesPerSecond?: number; diskBytesPerSecond?: number; diskMeasured?: boolean; activeConnections?: number; connectionLimit?: number; total: number; downloaded: number; bytesPerSecond: number; status: string; chunks: Chunk[] }
export interface Job { url?: string; missing?: boolean; rateLimit?: number; restored?: boolean; id: string; filename: string; path: string; category: string; createdAt: string; revision: number; workers: number; status: Status; error: string; progress: Progress; elapsedSeconds: number; etaSeconds: number; sourceKind: string }
export interface Request { url: string; filename: string; folder: string; workers: number }
export interface Settings { concurrent: number; rateLimit: number; notifications: boolean }
export interface State { settings: Settings; historyError?: string; jobs: Job[]; folder: string }
export const isActive = (status: string) => ['queued', 'probing', 'downloading', 'paused', 'canceling'].includes(status);
export const statusLabels: Record<string, string> = { probing: 'Холбогдож байна', downloading: 'Татаж байна', paused: 'Түр зогссон', canceling: 'Цуцалж байна', complete: 'Дууссан', failed: 'Алдаа гарсан', canceled: 'Цуцалсан', queued: 'Хүлээж байна', retrying: 'Дахин оролдож байна' };
export const duration = (seconds: number = 0) => { const s = Math.max(0, Math.floor(seconds)); return [Math.floor(s/3600), Math.floor(s/60)%60, s%60].map(n=>String(n).padStart(2,'0')).join(':'); };
export const remaining = (seconds: number) => !Number.isFinite(seconds) || seconds < 0 ? 'Тооцоолж байна…' : seconds < 60 ? `${Math.ceil(seconds)} сек` : seconds < 3600 ? `${Math.ceil(seconds/60)} мин` : `${Math.floor(seconds/3600)} цаг ${Math.ceil(seconds%3600/60)} мин`;
export const bytes = (value: number) => {
  if (value < 0) return 'Тодорхойгүй';
  if (value < 1024) return `${value.toFixed(0)} B`;
  const exponent = Math.min(Math.floor(Math.log(value) / Math.log(1024)), 4);
  return `${(value / 1024 ** exponent).toFixed(exponent > 1 ? 1 : 0)} ${['B', 'KiB', 'MiB', 'GiB', 'TiB'][exponent]}`;
};
export const percent = (job: Job) => job.status === 'complete' ? 100 : job.progress.total > 0 ? Math.min(100, job.progress.downloaded / job.progress.total * 100) : 0;
