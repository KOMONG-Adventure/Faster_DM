import { memo, useEffect, useRef, useState } from 'react';
import { ChartNoAxesColumnIncreasing, HardDrive } from 'lucide-react';

const rate = (bytes: number, bits = false) => {
  const value = Math.max(0, bytes) * (bits ? 8 : 1);
  const unit = bits ? ['bps', 'Kbps', 'Mbps', 'Gbps'] : ['B/s', 'KB/s', 'MB/s', 'GB/s'];
  const index = value > 0 ? Math.min(3, Math.floor(Math.log(value) / Math.log(1000))) : 0;
  const exponent = Math.max(0, index);
  return `${(value / 1000 ** exponent).toFixed(exponent ? 1 : 0)} ${unit[exponent]}`;
};
type Sample = { network: number; disk: number; diskKnown: boolean };

// Dashboard-ийн 30 fps event-ээс тусдаа, графикийг секундэд хоёр удаа шинэчилнэ.
export const TransferStats = memo(function TransferStats(props: Sample) {
  const latest = useRef(props);
  const [display, setDisplay] = useState({ ...props, peak: 0, history: Array<Sample>(60).fill({ network: 0, disk: 0, diskKnown: true }) });
  useEffect(() => { latest.current = props; }, [props.network, props.disk, props.diskKnown]);
  useEffect(() => {
    const timer = window.setInterval(() => {
      const sample = latest.current;
      setDisplay(previous => ({ ...sample, peak: Math.max(previous.peak, sample.network), history: [...previous.history.slice(1), sample] }));
    }, 500);
    return () => clearInterval(timer);
  }, []);
  const ceiling = Math.max(1, ...display.history.flatMap(s => [s.network, s.diskKnown ? s.disk : 0]));
  const points = (key: 'network' | 'disk') => display.history.map((s, i) => `${i * 1000 / 59},${100 - Math.min(1, s[key] / ceiling) * 90}`).join(' ');
  return <section className="transfer-panel" aria-label="Сүлжээ болон дискний хурд">
    <div className="transfer-metrics">
      <div><ChartNoAxesColumnIncreasing size={19}/><div><span>NETWORK <small>Сүлжээ</small></span><strong>{rate(display.network, true)}</strong></div></div>
      <div><ChartNoAxesColumnIncreasing size={19}/><div><span>PEAK <small>Дээд хурд</small></span><strong>{rate(display.peak, true)}</strong></div></div>
      <div className="disk-metric"><HardDrive size={18}/><div><span>DISK USAGE <small>Бичилт</small></span><strong>{display.diskKnown ? rate(display.disk) : '—'}</strong></div></div>
    </div>
    <svg className="transfer-graph" viewBox="0 0 1000 104" preserveAspectRatio="none" aria-hidden="true">
      {[25, 50, 75, 100].map(y => <line key={y} x1="0" y1={y} x2="1000" y2={y} className="graph-grid"/>)}
      <polygon points={`0,104 ${points('network')} 1000,104`} className="network-fill"/>
      <polyline points={points('network')} className="network-line"/>
      {display.history.every(s => s.diskKnown) ? <polyline points={points('disk')} className="disk-line"/> : null}
    </svg>
    <div className="transfer-caption"><span>Сүүлийн 30 секунд · Бүх таталтын нийлбэр</span><span>{display.diskKnown ? 'Диск: аппын бичилт · OS cache багтана' : 'YouTube-ийн дискний хурд хэмжигдэхгүй'}</span></div>
  </section>;
});
