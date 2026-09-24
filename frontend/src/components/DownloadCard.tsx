import { memo } from 'react';
import { Check, ChevronRight, FolderOpen, X, LoaderCircle, Pause, Play, Clock3 } from 'lucide-react';
import { categoryByKey } from '../categories';
import { bytes, duration, remaining, isActive, percent, statusLabels, type Job } from '../types';

export const DownloadCard = memo(function DownloadCard({ job, selected, onSelect, onCancel, onOpen, onPause, onResume }: { job: Job; selected: boolean; onSelect: (id: string) => void; onCancel: (id: string) => void; onOpen: (id: string) => void; onPause: (id: string) => void; onResume: (id: string) => void }) {
  const category = categoryByKey(job.category);
  const Icon = category.icon;
  const active = isActive(job.status);
  return <article className={`download-card ${selected ? 'selected' : ''}`}>
    <button className="card-select" onClick={() => onSelect(job.id)} aria-label={`${job.filename} дэлгэрэнгүй`} aria-pressed={selected}><span className={`file-icon ${category.className}`}><Icon size={22}/></span><span className="min-w-0 flex-1"><strong className="block truncate">{job.filename}</strong><span className="file-meta">{category.label}<span>•</span>{bytes(job.progress.total)}</span></span><span className={`status ${job.status}`}>{job.status === 'complete' ? <Check size={12}/> : job.status === 'probing' ? <LoaderCircle size={12} className="animate-spin"/> : null}{statusLabels[job.status]}</span><ChevronRight size={16} className="text-slate-400"/></button>
    <div className="card-progress"><div className="progress-topline"><strong>{percent(job).toFixed(1)}%</strong><span>{bytes(job.progress.bytesPerSecond)}/s</span></div><div className={`progress-track ${job.status}`} role="progressbar" aria-label={`${job.filename} таталтын явц`} aria-valuemin={0} aria-valuemax={100} aria-valuenow={job.progress.total >= 0 ? Math.round(percent(job)) : undefined}><div style={{ width: `${percent(job)}%` }}/></div><div className="progress-caption"><span>{bytes(job.progress.downloaded)} <span className="muted">/ {bytes(job.progress.total)}</span></span><span><Clock3 size={11}/> {duration(job.elapsedSeconds)}</span></div>{active ? <div className="eta-caption">{job.status==='paused' ? 'Range дэмждэг серверт татсан байрлалаас үргэлжилнэ.' : `Дуусах хүртэл: ${remaining(job.etaSeconds)}`}</div> : null}</div>
    {job.error ? <p className="card-error">{job.error}</p> : null}
    <div className="card-footer"><span className="truncate" title={job.path}><FolderOpen size={13}/> {job.category} / {job.filename}</span>{active ? <div className="download-actions">{job.status==='paused' ? <button className="resume-button" onClick={()=>onResume(job.id)}><Play size={13}/> Үргэлжлүүлэх</button> : <button disabled={job.status==='canceling'} onClick={()=>onPause(job.id)}><Pause size={13}/> Түр зогсоох</button>}<button disabled={job.status === 'canceling'} onClick={() => onCancel(job.id)} aria-label={`${job.filename} цуцлах`} title="Таталтыг бүрэн цуцлах"><X size={13}/></button></div> : <button onClick={() => onOpen(job.id)}><FolderOpen size={13}/> Хавтас нээх</button>}</div>
  </article>;
});
