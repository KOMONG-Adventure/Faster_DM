import { useEffect, useRef } from 'react';
import { Compass } from 'lucide-react';
import { driver, type Driver } from 'driver.js';
import 'driver.js/dist/driver.css';

const storageKey = 'fasterdm.welcome-guide.v1';

export function WelcomeGuide() {
  const guide = useRef<Driver>();
  useEffect(() => {
    let disposing = false;
    const tour = driver({
      animate: !window.matchMedia('(prefers-reduced-motion: reduce)').matches,
      showProgress: true, progressText: '{{current}} / {{total}}',
      nextBtnText: 'Дараах →', prevBtnText: '← Буцах', doneBtnText: 'Дуусгах',
      overlayColor: '#10203a', overlayOpacity: 0.65, stagePadding: 8, stageRadius: 12,
      popoverClass: 'faster-guide', disableActiveInteraction: true,
      onPopoverRender: popover => { popover.closeButton.setAttribute('aria-label', 'Танилцуулгыг алгасах'); },
      onDestroyed: () => { if (!disposing) { try { localStorage.setItem(storageKey, 'seen'); } catch { /* storage хаалттай үед апп ажиллана */ } } },
      steps: [
        { popover: { title: 'Faster DM ашиглах заавар', description: 'Үндсэн үйлдлүүдтэй танилцана уу. Esc дарж алгасаж болно.' } },
        { element: '.url-row', popover: { title: 'Татах холбоос', description: 'Файлын эсвэл YouTube видеоны холбоос оруулаад “Татаж эхлэх” дарна. Файлын нэрийг автоматаар олно.', side: 'bottom' } },
        { element: '.form-details', popover: { title: 'Холболт ба хадгалах хавтас', description: '“Автомат · 4–32” горим том файлын холболтыг тохируулна. Видео, зураг, ZIP файлууд өөрсдийн хавтастай. Нэрийг хүсвэл өөрчилж болно.' } },
        { element: '.transfer-panel', popover: { title: 'Хурдны үзүүлэлт', description: 'Сүлжээний хурд, дээд хурд, дискний бичилт болон сүүлийн 30 секундын график.' } },
        { element: '.list-section', popover: { title: 'Таталтын удирдлага', description: 'Таталт эхэлмэгц хувь, үлдсэн хугацаа, түр зогсоох болон үргэлжлүүлэх товч гарна. Апп бүрэн хаагдвал дутуу таталтыг дараа сэргээхгүй; ард татуулахдаа Tray ашиглаарай.' } },
        { element: '#tray-button', popover: { title: 'Tray-д хураах', description: '“Tray-д хураах” эсвэл цонхны − товчийг дарна. Цагийн хажуугийн Faster DM icon дээр дарж буцааж нээнэ. Icon далд байвал ∧ дотор харна. Баруун товч → “Аппаас гарах” нь бүрэн хаана.' } },
      ],
    });
    guide.current = tour;
    let seen = false;
    try { seen = localStorage.getItem(storageKey) === 'seen'; } catch { /* анхны танилцуулгыг харуулна */ }
    const timer = seen ? undefined : window.setTimeout(() => tour.drive(), 500);
    return () => { disposing = true; clearTimeout(timer); tour.destroy(); guide.current = undefined; };
  }, []);
  return <button className="header-tool" onClick={() => { if (!guide.current?.isActive()) guide.current?.drive(); }}><Compass size={15}/> Заавар</button>;
}
