import { useEffect, useRef, useState, type FormEvent } from 'react';
import { ArrowDownToLine, FolderOpen, Link2, ArrowRight, LoaderCircle } from 'lucide-react';
import { api, desktop, type DiskReport } from '../bridge';
import { categoryFor, suggestName } from '../categories';
import { bytes, type Job } from '../types';

export function AddDownload({ folder, setFolder, onCreated }: { folder: string; setFolder: (folder: string) => void; onCreated: (job: Job) => void }) {
  const [url, setURLState] = useState('');
  const currentURL = useRef('');
  function setURL(value: string) { currentURL.current = value; setURLState(value); }
  const [report, setReport] = useState<{key: string; data: DiskReport} | null>(null);
  const [filename, setFilename] = useState('');
  const [workers, setWorkers] = useState(0);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [autoClipboard, setAutoClipboard] = useState(() => { try { return localStorage.getItem('fasterdm.clipboard-auto') !== 'off'; } catch { return true; } });
  const [clipboardNotice, setClipboardNotice] = useState('');
  const requestKey=JSON.stringify([url,filename,folder]);
  const disk=report?.key===requestKey ? report.data : null;
  useEffect(()=>{
    if(!desktop || busy || url) return;
    let disposed=false,reading=false;
    async function poll(){if(reading || disposed)return;reading=true;try{const draft=await api().GetBrowserLink();if(disposed || currentURL.current || !draft.id)return;setURL(draft.url);setClipboardNotice('Браузераас холбоос ирлээ.');await api().AcceptBrowserLink(draft.id);}catch(err){if(!disposed)setError(String(err));}finally{reading=false;}}
    void poll();const timer=window.setInterval(poll,1000);return()=>{disposed=true;clearInterval(timer);};
  },[busy,url]);
  const lastClipboard = useRef('');
  const autoFilled = useRef('');
  useEffect(() => {
    if (!desktop || !autoClipboard || busy || !api().ClipboardDownloadURL) return;
    let disposed = false;
    let reading = false;
    async function poll() {
      if (disposed || reading || document.hidden) return;
      reading = true;
      try {
        const copied = await api().ClipboardDownloadURL();
        if (disposed || copied === lastClipboard.current) return;
        lastClipboard.current = copied;
        if (copied && (!currentURL.current || currentURL.current === autoFilled.current)) {
          autoFilled.current = copied;
          setURL(copied);
          setClipboardNotice('Холбоос автоматаар орлоо.');
        }
      } catch { /* Clipboard өөр аппд түр түгжигдвэл дараагийн мөчлөгт оролдоно. */ }
      finally { reading = false; }
    }
    void poll();
    const timer = window.setInterval(poll, 900);
    window.addEventListener('focus', poll);
    document.addEventListener('visibilitychange', poll);
    return () => { disposed = true; clearInterval(timer); window.removeEventListener('focus', poll); document.removeEventListener('visibilitychange', poll); };
  }, [autoClipboard, busy, url]);
  function toggleClipboard(enabled: boolean) {
    if (enabled) lastClipboard.current = '';
    setAutoClipboard(enabled);
    setClipboardNotice('');
    try { localStorage.setItem('fasterdm.clipboard-auto', enabled ? 'on' : 'off'); } catch { /* энэ удаагийн тохиргоо хүчинтэй */ }
  }
  async function pasteLink() {
    try {
      if (!api().ClipboardDownloadURL) throw new Error('Шинэ EXE-г нээнэ үү.');
      const copied = await api().ClipboardDownloadURL();
      if (!copied) { setClipboardNotice('Clipboard дотор HTTP/HTTPS холбоос алга.'); return; }
      lastClipboard.current = copied;
      autoFilled.current = copied;
      setURL(copied);
      setClipboardNotice('Хуулсан холбоос орлоо.');
    } catch { setClipboardNotice('Clipboard уншиж чадсангүй. Ctrl+V ашиглана уу.'); }
  }
  const youtube = /^(https?:\/\/)?(www\.|m\.)?(youtube\.com|youtu\.be)\//i.test(url);
  const category = categoryFor(youtube ? 'video.mp4' : filename || suggestName(url));
  async function chooseFolder() {
    try { const selected = await api().ChooseFolder(); if (selected) setFolder(selected); }
    catch (err) { setError(String(err)); }
  }
  async function submit(event: FormEvent) {
    event.preventDefault();
    if (busy) return;
    setBusy(true); setError(''); setReport(null);
    try {
      const data = await api().CheckDiskSpace({ url, filename, folder, workers });
      if (!data.enough) {
        setReport({ key: requestKey, data });
        setError('Дискний зай хүрэлцэхгүй. Өөр хавтас сонгох эсвэл зай гаргаад дахин татна уу.');
        return;
      }
      const job = await api().StartDownload({ url, filename: data.filename, folder, workers }); onCreated(job); setURL(''); setFilename(''); autoFilled.current = ''; setClipboardNotice(''); }
    catch (err) { setError(String(err)); }
    finally { setBusy(false); }
  }
  return <section className="add-panel" aria-labelledby="new-download-title">
    <div className="flex items-center justify-between gap-3 mb-5"><div className="flex items-center gap-2.5"><span className="mini-icon"><Link2 size={17} /></span><h2 id="new-download-title">Шинэ таталт</h2></div></div>
    <form onSubmit={submit}>
      <label className="sr-only" htmlFor="source-url">Татах холбоос</label>
      <div className="url-row"><Link2 className="shrink-0 text-slate-400" size={19}/><input id="source-url" type="url" required autoComplete="off" placeholder="Файл эсвэл YouTube холбоос" value={url} onChange={e => { autoFilled.current = ''; setClipboardNotice(''); setURL(e.target.value); }} /><button className="primary-button" disabled={busy || !desktop}>{busy ? <LoaderCircle size={17} className="animate-spin" /> : <ArrowDownToLine size={17} />} {busy ? 'Эхлүүлж байна…' : 'Татаж эхлэх'}</button></div>
      <div className="clipboard-options"><label><input type="checkbox" checked={autoClipboard} disabled={!desktop} onChange={e=>toggleClipboard(e.target.checked)}/> Хуулсан холбоосыг автоматаар оруулах</label><button type="button" disabled={!desktop || busy} onClick={pasteLink}>Хуулсан холбоос оруулах</button></div>
      {clipboardNotice ? <p className="clipboard-notice" role="status">{clipboardNotice}</p> : null}
      <div className="form-details"><div className="filename-field"><label htmlFor="filename">Файлын нэр</label><input id="filename" maxLength={180} placeholder="Автоматаар тодорхойлно" value={filename} onChange={e => setFilename(e.target.value)} /></div><div><label htmlFor="connections">Зэрэгцээ холболт</label><select id="connections" value={workers} onChange={e => setWorkers(Number(e.target.value))}><option value={0}>Автомат · 1–32</option>{[1,2,4,8,16,32].map(n => <option key={n} value={n}>{n} холболт</option>)}</select></div><button type="button" className="folder-picker" onClick={chooseFolder} disabled={!desktop}><FolderOpen size={17}/> Хавтас сонгох</button></div>
      <div className="save-preview"><FolderOpen size={14}/><input aria-label="Хадгалах үндсэн хавтас" className="folder-path" title={folder} placeholder="Хадгалах үндсэн хавтас" value={folder} onChange={e=>setFolder(e.target.value)}/><ArrowRight size={12}/><strong>{category.key}</strong>{filename ? <><ArrowRight size={12}/><span className="truncate">{filename}</span></> : null}</div>
      {disk && !disk.enough ? <div className={`disk-report ${disk.enough?'':'low'}`} role="status"><strong>{disk.filename}</strong><span>Файл: {bytes(disk.total)} · Сул зай: {bytes(disk.free)} · Шаардлага: {bytes(disk.required)}</span><small>{disk.estimated ? 'Видео нэгтгэх түр зайг тооцсон ойролцоо хэмжээ.' : disk.total<0 ? 'Сервер хэмжээг өгөөгүй. Таталтын явцад сул зайг хянана.' : '67.1 MB нөөц зай багтсан. Эхлэх үед дахин шалгана.'}</small></div> : null}
      {error ? <p role="alert" className="form-error">{error}</p> : null}
    </form>
  </section>;
}
