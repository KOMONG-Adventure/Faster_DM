import { useState, type FormEvent } from 'react';
import { ArrowDownToLine, FolderOpen, Link2, ArrowRight, Layers3, LoaderCircle } from 'lucide-react';
import { api, desktop } from '../bridge';
import { categoryFor, suggestName } from '../categories';
import type { Job } from '../types';

export function AddDownload({ folder, setFolder, onCreated }: { folder: string; setFolder: (folder: string) => void; onCreated: (job: Job) => void }) {
  const [url, setURL] = useState('');
  const [filename, setFilename] = useState('');
  const [workers, setWorkers] = useState(16);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const youtube = /^(https?:\/\/)?(www\.|m\.)?(youtube\.com|youtu\.be)\//i.test(url);
  const category = categoryFor(youtube ? 'video.mp4' : filename || suggestName(url));
  async function chooseFolder() {
    try { const selected = await api().ChooseFolder(); if (selected) setFolder(selected); }
    catch (err) { setError(String(err)); }
  }
  async function submit(event: FormEvent) {
    event.preventDefault();
    if (busy) return;
    setBusy(true); setError('');
    try { const job = await api().StartDownload({ url, filename, folder, workers }); onCreated(job); setURL(''); setFilename(''); }
    catch (err) { setError(String(err)); }
    finally { setBusy(false); }
  }
  return <section className="add-panel" aria-labelledby="new-download-title">
    <div className="flex items-center justify-between gap-3 mb-5"><div className="flex items-center gap-2.5"><span className="mini-icon"><Link2 size={17} /></span><h2 id="new-download-title">Шинэ таталт</h2></div><span className="auto-tag"><Layers3 size={13} /> Автоматаар ангилна</span></div>
    <form onSubmit={submit}>
      <label className="sr-only" htmlFor="source-url">Татах холбоос</label>
      <div className="url-row"><Link2 className="shrink-0 text-slate-400" size={19}/><input id="source-url" type="url" required autoComplete="off" placeholder="Файлын эсвэл YouTube холбоос оруулна уу…" value={url} onChange={e => { setURL(e.target.value); }} /><button className="primary-button" disabled={busy || !desktop}>{busy ? <LoaderCircle size={17} className="animate-spin" /> : <ArrowDownToLine size={17} />} {busy ? 'Мэдээлэл авч байна…' : 'Татаж эхлэх'}</button></div>
      <div className="form-details"><div className="filename-field"><label htmlFor="filename">Файлын нэр · автоматаар</label><input id="filename" maxLength={180} placeholder="Бичих шаардлагагүй — автоматаар нэрлэнэ" value={filename} onChange={e => setFilename(e.target.value)} /></div><div><label htmlFor="connections">Зэрэгцээ холболт</label><select id="connections" value={workers} onChange={e => setWorkers(Number(e.target.value))}>{[4,8,16,32].map(n => <option key={n} value={n}>{n} холболт</option>)}</select></div><button type="button" className="folder-picker" onClick={chooseFolder} disabled={!desktop}><FolderOpen size={17}/> Хавтас сонгох</button></div>
      <div className="save-preview"><FolderOpen size={14}/><input aria-label="Хадгалах үндсэн хавтас" className="folder-path" title={folder} placeholder="Хадгалах үндсэн хавтас" value={folder} onChange={e=>setFolder(e.target.value)}/><ArrowRight size={12}/><strong>{category.key}</strong>{filename ? <><ArrowRight size={12}/><span className="truncate">{filename}</span></> : null}</div>
      {error ? <p role="alert" className="form-error">{error}</p> : null}
    </form>
  </section>;
}
