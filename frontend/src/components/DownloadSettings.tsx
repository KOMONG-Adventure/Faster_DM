import { useState } from 'react';
import { api, desktop } from '../bridge';
import type { Settings } from '../types';

export function DownloadSettings({ settings, onSaved, onError }: { settings: Settings; onSaved: (s: Settings) => void; onError: (message: string) => void }) {
  const [concurrent, setConcurrent] = useState(settings.concurrent);
  const [rate, setRate] = useState(String(settings.rateLimit / 1048576));
  const [notifications, setNotifications] = useState(settings.notifications);
  const [busy, setBusy] = useState(false);
  async function save() {
    const value = Number(rate);
    if (!Number.isFinite(value) || value < 0 || value > 1024) { onError('Хурд 0–1024 MiB/s байна.'); return; }
    const next = { concurrent, rateLimit: Math.round(value * 1048576), notifications };
    setBusy(true);
    try { await api().SetDownloadSettings(next); onSaved(next); } catch (err) { onError(String(err)); } finally { setBusy(false); }
  }
  return <details className="download-settings"><summary>Таталтын тохиргоо · Зэрэг {settings.concurrent} · {settings.rateLimit ? `${settings.rateLimit / 1048576} MiB/s / файл` : 'Хязгааргүй'}</summary>
    <div className="settings-fields">
      <label>Зэрэг татах файл<select value={concurrent} onChange={e => setConcurrent(Number(e.target.value))}>{[1,2,3,4].map(n => <option key={n} value={n}>{n}</option>)}</select></label>
      <label>Нэг файлын хурд (MiB/s)<input type="number" min="0" max="1024" step="0.1" value={rate} onChange={e => setRate(e.target.value)}/></label>
      <label className="notification-toggle"><input type="checkbox" checked={notifications} onChange={e => setNotifications(e.target.checked)}/> Дуусах / алдааны мэдэгдэл</label>
      <button disabled={!desktop || busy} onClick={save}>{busy ? 'Хадгалж байна…' : 'Хадгалах'}</button>
    </div><p>0 = хязгааргүй. Хурдны шинэ утга дараагийн эхлэх болон үргэлжлүүлэх таталтад үйлчилнэ. Зэрэг таталтын тоог бууруулбал одоогийн таталтууд дууссаны дараа мөрдөнө.</p>
  </details>;
}
