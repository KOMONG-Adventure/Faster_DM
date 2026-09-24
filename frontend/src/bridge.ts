import type { Job, Request, State, Settings } from './types';

interface API {
 SetDownloadSettings(settings: Settings): Promise<void>;
 RemoveHistory(id: string): Promise<void>;
 RetryDownload(id: string): Promise<Job>;
 OpenFile(id: string): Promise<void>;
	ClipboardDownloadURL(): Promise<string>;
	MinimiseToTray(): Promise<void>;
  GetAppVersion(): Promise<string>;
  CheckForUpdates(): Promise<UpdateResult>;
  InstallUpdate(tag: string): Promise<void>;
  GetUpdateProgress(): Promise<UpdateProgress>;
  GetState(): Promise<State>;
  StartDownload(request: Request): Promise<Job>;
  CancelDownload(id: string): Promise<void>;
  PauseDownload(id: string): Promise<void>;
  ResumeDownload(id: string): Promise<void>;
  ChooseFolder(): Promise<string>;
  OpenFolder(id: string): Promise<void>;
}
export interface UpdateResult { current: string; latest: string; status: 'available' | 'current' | 'unavailable' | 'error'; message: string; checkedAt: string; publishedAt: string }
export interface UpdateProgress { phase: string; downloaded: number; total: number }
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
