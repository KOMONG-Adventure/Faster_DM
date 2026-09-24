import type { Job, Request, State } from './types';

interface API {
	ClipboardDownloadURL(): Promise<string>;
	MinimiseToTray(): Promise<void>;
  GetAppVersion(): Promise<string>;
  CheckForUpdates(): Promise<UpdateResult>;
  OpenReleases(): Promise<void>;
  GetState(): Promise<State>;
  StartDownload(request: Request): Promise<Job>;
  CancelDownload(id: string): Promise<void>;
  PauseDownload(id: string): Promise<void>;
  ResumeDownload(id: string): Promise<void>;
  ChooseFolder(): Promise<string>;
  OpenFolder(id: string): Promise<void>;
}
export interface UpdateResult { current: string; latest: string; status: 'available' | 'current' | 'unavailable' | 'error'; message: string; checkedAt: string; publishedAt: string }
declare global {
  interface Window {
    go?: { main: { App: API } };
    runtime?: { EventsOnMultiple(name: string, callback: (job: Job) => void, count: number): () => void };
  }
}
export const desktop = Boolean(window.go?.main?.App);
export function api(): API {
  const backend = window.go?.main?.App;
  if (!backend) throw new Error('Таталт эхлүүлэхийн тулд FasterDM.exe аппыг нээнэ үү.');
  return backend;
}
export function subscribe(callback: (job: Job) => void): () => void {
  return window.runtime?.EventsOnMultiple('download:changed', callback, -1) ?? (() => {});
}
