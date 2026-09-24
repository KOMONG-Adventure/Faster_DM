import { useCallback, useEffect, useRef, useState } from 'react';
import { motion, MotionConfig } from 'framer-motion';
import { ArrowDownToLine, ArrowUpRight, CheckCheck, Download, FolderClosed, Gauge, LayoutDashboard, Search, ShieldCheck, X, Zap } from 'lucide-react';
import { api, desktop, subscribe } from './bridge';
import { categories } from './categories';
import { bytes, isActive, type Job } from './types';
import { AddDownload } from './components/AddDownload';
import { DownloadCard } from './components/DownloadCard';
import { ChunkPanel } from './components/ChunkPanel';
import { TransferStats } from './components/TransferStats';
import { UpdateChecker } from './components/UpdateChecker';

function merge(jobs: Job[], incoming: Job): Job[] {
  const existing = jobs.find(j => j.id === incoming.id);
  if (existing && existing.revision >= incoming.revision) return jobs;
  return existing ? jobs.map(j => j.id === incoming.id ? incoming : j) : [incoming, ...jobs];
}

export default function App() {
  const [jobs, setJobs] = useState<Job[]>([]);
  const [folder, setFolder] = useState('');
  const [category, setCategory] = useState('all');
  const [filter, setFilter] = useState('all');
  const [query, setQuery] = useState('');
  const [selected, setSelected] = useState('');
  const [error, setError] = useState('');
  const pending = useRef(new Map<string, Job>());
  useEffect(() => {
    if (!desktop) return;
    let live = true;
    const unsubscribe = subscribe(job => { const previous = pending.current.get(job.id); if (!previous || previous.revision < job.revision) pending.current.set(job.id,job); });
    // Нэг frame-д ирсэн олон snapshot-ийг нэг React update болгон нэгтгэнэ.
    const timer = window.setInterval(() => { if (!pending.current.size) return; const batch=[...pending.current.values()]; pending.current.clear(); setJobs(previous=>batch.reduce(merge,previous)); }, 33);
    api().GetState().then(state => { if (!live) return; setFolder(state.folder); setJobs(previous=>state.jobs.reduce(merge,previous)); }).catch(err=>{ if(live) setError(String(err)); });
    return () => { live=false; unsubscribe(); clearInterval(timer); };
  }, []);
  const onCreated = useCallback((job: Job) => { setJobs(previous=>merge(previous,job)); setSelected(job.id); setCategory('all'); setFilter('all'); setQuery(''); }, []);
  const onCancel = useCallback((id: string) => { api().CancelDownload(id).catch(err=>setError(String(err))); }, []);
  const onOpen = useCallback((id: string) => { api().OpenFolder(id).catch(err=>setError(String(err))); }, []);
  const onPause = useCallback((id: string) => { if (!api().PauseDownload) { setError('Энэ нээлттэй апп хуучин хувилбар байна. Одоогийн таталтаа дуусгаад шинэ FasterDM.exe-г нээнэ үү.'); return; } api().PauseDownload(id).catch(err=>setError(String(err))); }, []);
  const onResume = useCallback((id: string) => { api().ResumeDownload(id).catch(err=>setError(String(err))); }, []);
  const active = jobs.filter(j=>isActive(j.status) && j.status!=='paused');
  const completed = jobs.filter(j=>j.status==='complete');
  const totalSpeed = active.reduce((sum,j)=>sum+j.progress.bytesPerSecond,0);
  const visible = jobs.filter(j=>(category==='all'||j.category===category) && (filter==='all'||filter==='active'&&isActive(j.status)||filter==='complete'&&j.status==='complete'||filter==='failed'&&(j.status==='failed'||j.status==='canceled')) && j.filename.toLowerCase().includes(query.toLowerCase()));
  const current = jobs.find(j=>j.id===selected) ?? visible[0];
  const title = category==='all' ? 'Таталтын самбар' : categories.find(c=>c.key===category)?.label;
  return <MotionConfig reducedMotion="user"><div className="app-shell">
    <nav className="sidebar" aria-label="Үндсэн цэс"><div className="brand"><span className="brand-mark"><Zap size={23} fill="currentColor"/></span><div>faster<span>DOWNLOAD MANAGER</span></div></div><p className="nav-label">АЖЛЫН ТАЛБАР</p><button className={`nav-item ${category==='all'?'active':''}`} onClick={()=>{setCategory('all');setFilter('all');}}><LayoutDashboard size={18}/> Бүх таталт <span>{jobs.length}</span></button><div className="nav-divider"/><p className="nav-label">ФАЙЛЫН АНГИЛАЛ</p><div className="category-nav">{categories.map(c=><button key={c.key} className={`nav-item ${category===c.key?'active':''}`} onClick={()=>{setCategory(c.key);setFilter('all');}}><c.icon size={17}/>{c.label}<span>{jobs.filter(j=>j.category===c.key).length}</span></button>)}</div><div className="sidebar-bottom"><div className="organize-note"><FolderClosed size={19}/><strong>Бүх зүйл байрандаа.</strong><p>Татсан файлууд төрлөөрөө хавтаслагдана.</p><div className="note-folders"><span>MP4</span><span>JPG</span><span>ZIP</span></div></div><UpdateChecker/></div></nav>
    <div className="workspace"><header className="topbar"><div><span className="breadcrumb">Миний файлууд</span><span className="breadcrumb-slash">/</span><span>Таталтууд</span></div><div className="ready-indicator"><span className="live-dot"/>{desktop?'Холбогдсон':'Загварын харагдац'}</div></header><main className="main-content"><div className="page-heading"><div><p className="eyebrow">ХУРДТАЙ. ЦЭГЦТЭЙ. ТАНЫ ХЯНАЛТАД.</p><h1>{title}</h1><p>Таталтаа нэг дороос удирдаарай.</p></div><span className="engine-badge"><Zap size={14}/> Олон холболттой таталт</span></div>
    {!desktop ? <div className="preview-banner">Энэ нь браузерын харагдац. Файл татахын тулд FasterDM.exe аппыг нээнэ үү.</div> : null}
    {error ? <div className="global-error" role="alert">{error}<button onClick={()=>setError('')} aria-label="Мэдэгдэл хаах"><X size={16}/></button></div> : null}
    <div className="stats-grid"><div className="stat"><div><span>Нийт таталт</span><strong>{jobs.length}<small>файл</small></strong></div><span className="stat-icon slate"><Download size={20}/></span></div><div className="stat"><div><span>Идэвхтэй</span><strong>{active.length}<small>таталт</small></strong></div><span className="stat-icon blue"><ArrowDownToLine size={20}/></span></div><div className="stat"><div><span>Дууссан</span><strong>{completed.length}<small>файл</small></strong></div><span className="stat-icon teal"><CheckCheck size={20}/></span></div><div className="stat"><div><span>Нийт хурд</span><strong className="speed-stat">{bytes(totalSpeed)}<small>/s</small></strong></div><span className="stat-icon violet"><Gauge size={20}/></span></div></div>
    <TransferStats network={active.reduce((sum,j)=>sum+(j.sourceKind === 'youtube' ? j.progress.bytesPerSecond : j.progress.networkBytesPerSecond ?? 0),0)} disk={active.reduce((sum,j)=>sum+(j.progress.diskBytesPerSecond ?? 0),0)} diskKnown={active.every(j=>j.status === 'probing' || j.progress.diskMeasured === true)}/>
    <div className="content-grid"><div className="downloads-column"><AddDownload folder={folder} setFolder={setFolder} onCreated={onCreated}/><section className="list-section" aria-label="Таталтын жагсаалт"><div className="list-heading"><h2>Таны таталтууд <span>{visible.length}</span></h2><div className="search-field"><Search size={15}/><input aria-label="Файл хайх" placeholder="Файл хайх…" value={query} onChange={e=>setQuery(e.target.value)}/></div></div><div className="filter-tabs" role="group" aria-label="Төлөвөөр шүүх">{[['all','Бүгд'],['active','Идэвхтэй'],['complete','Дууссан'],['failed','Зогссон']].map(([key,label])=><button key={key} aria-pressed={filter===key} className={filter===key?'active':''} onClick={()=>setFilter(key)}>{label}</button>)}</div><div className="download-list">{visible.length ? visible.map(job=><DownloadCard key={job.id} job={job} selected={current?.id===job.id} onSelect={setSelected} onCancel={onCancel} onOpen={onOpen} onPause={onPause} onResume={onResume}/>) : <motion.div initial={{opacity:0,y:8}} animate={{opacity:1,y:0}} className="empty-state"><div className="empty-graphic"><span className="floating-file file-one"><span>ZIP</span></span><span className="floating-file file-two"><span>MP4</span></span><span className="download-orbit"><ArrowDownToLine size={34}/></span></div><h3>{jobs.length?'Энд харуулах таталт алга':'Анхны таталтаа эхлүүлээрэй'}</h3><p>{jobs.length?'Хайлтын үг эсвэл шүүлтүүрээ өөрчилнө үү.':'Дээрх талбарт файлын холбоосоо оруулна уу. Бид үлдсэнийг нь цэгцэлнэ.'}</p><span className="empty-hint"><ArrowUpRight size={15}/> {jobs.length?'Таталтуудаа ангиллаар харах боломжтой':'Видео, зураг, архив — тус бүр өөрийн хавтастай.'}</span></motion.div>}</div></section><footer className="workspace-footer"><ShieldCheck size={14}/> Байгаа файлыг дарж бичихгүй <span>•</span> Энэ удаагийн таталтууд</footer></div><ChunkPanel job={current}/></div></main></div>
  </div></MotionConfig>;
}
