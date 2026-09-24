import { useEffect, useRef, useState } from 'react';
import { Download, CheckCircle2, CircleAlert, RefreshCw, X } from 'lucide-react';
import { api, desktop, type UpdateResult, type UpdateProgress } from '../bridge';
import { bytes } from '../types';

export function UpdateChecker() {
  const dialog = useRef<HTMLDialogElement>(null);
  const running = useRef(false);
  const [version, setVersion] = useState('—');
  const [busy, setBusy] = useState(false);
  const [result, setResult] = useState<UpdateResult>();
  const [error, setError] = useState('');
  const [installing, setInstalling] = useState(false);
  const [progress, setProgress] = useState<UpdateProgress>({phase:'checking', downloaded:0, total:0});
  const working = busy || installing;
  useEffect(() => {
    if (!installing) return;
    let disposed = false;
    let pending = false;
    const timer = window.setInterval(async () => {
      if (pending) return;
      pending = true;
      try { const p = await api().GetUpdateProgress(); if (!disposed) setProgress(p); }
      catch { /* Апп installer рүү шилжиж хаагдаж болно. */ }
      finally { pending = false; }
    }, 400);
    return () => { disposed = true; clearInterval(timer); };
  }, [installing]);
  useEffect(() => {
    let live = true;
    if (desktop && api().GetAppVersion) api().GetAppVersion().then(v => { if (live) setVersion(v); }).catch(() => {});
    return () => { live = false; };
  }, []);
  async function check() {
    if (running.current) return;
    if (!dialog.current?.open) dialog.current?.showModal();
    setError('');
    if (!api().CheckForUpdates) { setError('Шинэчлэлт шалгагчтай шинэ FasterDM.exe-г нээнэ үү.'); return; }
    running.current = true; setBusy(true); setResult(undefined);
    try { setResult(await api().CheckForUpdates()); }
    catch { setError('Шинэчлэлт шалгаж чадсангүй. Дахин оролдоно уу.'); }
    finally { running.current = false; setBusy(false); }
  }
  async function install() {
    if (running.current || result?.status !== 'available') return;
    if (!api().InstallUpdate) { setError('Шууд суулгах боломжийг авахын тулд шинэ installer-ийг нэг удаа гараар суулгана уу.'); return; }
    running.current = true; setInstalling(true); setError(''); setProgress({phase:'checking',downloaded:0,total:0});
    try { await api().InstallUpdate(result.latest); }
    catch (err) { setError(String(err)); }
    finally { running.current = false; setInstalling(false); }
  }
  const installLabel = progress.phase === 'checking' ? 'Суулгагчийг шалгаж байна…' : progress.phase === 'verifying' ? 'Checksum шалгаж байна…' : progress.phase === 'installing' ? 'Суулгаж байна. Апп дахин нээгдэнэ…' : 'Шинэчлэлт татаж байна…';
  return <>
    <button className="update-check-button" onClick={check} disabled={!desktop || working}><RefreshCw size={14} className={working ? 'animate-spin' : ''}/>{installing ? 'Шинэчлэлт татаж байна…' : busy ? 'Шалгаж байна…' : 'Шинэчлэлт шалгах'}</button>
    <div className="app-version"><span className="live-dot"/> Faster DM <span>{version === '—' ? version : `v${version}`}</span></div>
    <dialog ref={dialog} className="update-dialog" aria-labelledby="update-title" onCancel={e=>{if(installing)e.preventDefault();}}>
      <div className="update-dialog-header"><h2 id="update-title">Аппын шинэчлэлт</h2><button disabled={installing} onClick={() => dialog.current?.close()} aria-label="Шинэчлэлтийн цонх хаах"><X size={19}/></button></div>
      <div className="update-body" aria-live="polite">
        <div className={`update-symbol ${result?.status === 'available' ? 'available' : ''}`}>{working ? <RefreshCw className="animate-spin" size={25}/> : error || result?.status === 'error' ? <CircleAlert size={25}/> : <CheckCircle2 size={25}/>}</div>
        <h3>{installing ? installLabel : busy ? 'Шинэ хувилбар шалгаж байна…' : result?.status === 'available' ? 'Шинэ хувилбар бэлэн!' : result?.status === 'current' ? 'Хувилбар шинэ байна' : 'Шалгалтын үр дүн'}</h3>
        <p>{installing ? 'Таталт дуусмагц апп хаагдаж, суулгасны дараа дахин нээгдэнэ.' : busy ? 'GitHub Releases-тэй холбогдож байна.' : error || result?.message}</p>
        {installing && progress.downloaded > 0 ? <div className="update-download-progress"><progress aria-label="Installer таталтын явц" max={progress.total > 0 ? progress.total : undefined} value={progress.total > 0 ? progress.downloaded : undefined}/><span>{bytes(progress.downloaded)}{progress.total > 0 ? ` / ${bytes(progress.total)} · ${Math.min(100,progress.downloaded/progress.total*100).toFixed(0)}%` : ''}</span></div> : null}
        <dl><div><dt>Одоогийн хувилбар</dt><dd>{result?.current || version}</dd></div>{result?.latest ? <div><dt>Нийтлэгдсэн хувилбар</dt><dd>{result.latest}</dd></div> : null}{result?.checkedAt ? <div><dt>Сүүлд шалгасан</dt><dd>{new Date(result.checkedAt).toLocaleString('mn-MN')}</dd></div> : null}</dl>
        <div className="update-actions"><button className="folder-picker" disabled={working} onClick={check}><RefreshCw size={14}/> Дахин шалгах</button><button className="primary-button" disabled={working || !desktop || result?.status !== 'available'} onClick={install}><Download size={15}/>{installing ? 'Түр хүлээнэ үү…' : 'Татаж суулгах'}</button></div>
      </div>
    </dialog>
  </>;
}
