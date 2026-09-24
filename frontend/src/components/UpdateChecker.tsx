import { useEffect, useRef, useState } from 'react';
import { ArrowUpRight, CheckCircle2, CircleAlert, RefreshCw, X } from 'lucide-react';
import { api, desktop, type UpdateResult } from '../bridge';

export function UpdateChecker() {
  const dialog = useRef<HTMLDialogElement>(null);
  const running = useRef(false);
  const [version, setVersion] = useState('—');
  const [busy, setBusy] = useState(false);
  const [result, setResult] = useState<UpdateResult>();
  const [error, setError] = useState('');
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
  async function openReleases() {
    try { await api().OpenReleases(); }
    catch { setError('GitHub Releases хуудсыг нээж чадсангүй.'); }
  }
  return <>
    <button className="update-check-button" onClick={check} disabled={!desktop || busy}><RefreshCw size={14} className={busy ? 'animate-spin' : ''}/>{busy ? 'Шалгаж байна…' : 'Шинэчлэлт шалгах'}</button>
    <div className="app-version"><span className="live-dot"/> Faster DM <span>{version === '—' ? version : `v${version}`}</span></div>
    <dialog ref={dialog} className="update-dialog" aria-labelledby="update-title">
      <div className="update-dialog-header"><h2 id="update-title">Аппын шинэчлэлт</h2><button onClick={() => dialog.current?.close()} aria-label="Шинэчлэлтийн цонх хаах"><X size={19}/></button></div>
      <div className="update-body" aria-live="polite">
        <div className={`update-symbol ${result?.status === 'available' ? 'available' : ''}`}>{busy ? <RefreshCw className="animate-spin" size={25}/> : error || result?.status === 'error' ? <CircleAlert size={25}/> : <CheckCircle2 size={25}/>}</div>
        <h3>{busy ? 'Шинэ хувилбар шалгаж байна…' : result?.status === 'available' ? 'Шинэ хувилбар бэлэн!' : result?.status === 'current' ? 'Хувилбар шинэ байна' : 'Шалгалтын үр дүн'}</h3>
        <p>{busy ? 'GitHub Releases-тэй холбогдож байна.' : error || result?.message}</p>
        <dl><div><dt>Одоогийн хувилбар</dt><dd>{result?.current || version}</dd></div>{result?.latest ? <div><dt>Нийтлэгдсэн хувилбар</dt><dd>{result.latest}</dd></div> : null}{result?.checkedAt ? <div><dt>Сүүлд шалгасан</dt><dd>{new Date(result.checkedAt).toLocaleString('mn-MN')}</dd></div> : null}</dl>
        <div className="update-actions"><button className="folder-picker" disabled={busy} onClick={check}><RefreshCw size={14}/> Дахин шалгах</button><button className="primary-button" disabled={busy || !desktop} onClick={openReleases}>GitHub Releases <ArrowUpRight size={15}/></button></div>
      </div>
    </dialog>
  </>;
}
