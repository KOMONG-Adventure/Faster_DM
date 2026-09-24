#ifndef AppVersion
  #error AppVersion required
#endif
#ifndef PackageDir
  #error PackageDir required
#endif
#ifndef ReleaseDir
  #error ReleaseDir required
#endif

[Setup]
AppId={{7442671B-FE16-48CB-8F03-89835AF7A8D4}
AppName=Faster DM
AppVersion={#AppVersion}
AppPublisher=KOMONG Adventure
AppPublisherURL=https://github.com/KOMONG-Adventure/Faster_DM
AppUpdatesURL=https://github.com/KOMONG-Adventure/Faster_DM/releases
DefaultDirName={localappdata}\Programs\FasterDM
DefaultGroupName=Faster DM
PrivilegesRequired=lowest
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
MinVersion=10.0
DisableProgramGroupPage=yes
DisableDirPage=yes
UsePreviousAppDir=no
OutputDir={#ReleaseDir}
OutputBaseFilename=FasterDM-Setup-{#AppVersion}-x64
SetupIconFile=icon.ico
UninstallDisplayIcon={app}\FasterDM.exe
Compression=lzma2/fast
SolidCompression=yes
WizardStyle=modern
CloseApplications=no
RestartApplications=no

[Tasks]
Name: desktopicon; Description: "Desktop дээр товчлол үүсгэх"; Flags: unchecked

[Files]
Source: "{#PackageDir}\FasterDM.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "{#PackageDir}\tools\*"; DestDir: "{app}\tools"; Flags: ignoreversion recursesubdirs createallsubdirs
Source: "{#PackageDir}\THIRD_PARTY.md"; DestDir: "{app}"; Flags: ignoreversion
Source: "{#PackageDir}\licenses\*"; DestDir: "{app}\licenses"; Flags: ignoreversion

[Icons]
Name: "{group}\Faster DM"; Filename: "{app}\FasterDM.exe"
Name: "{autodesktop}\Faster DM"; Filename: "{app}\FasterDM.exe"; Tasks: desktopicon

[Run]
Filename: "{app}\FasterDM.exe"; Description: "Faster DM нээх"; Flags: nowait postinstall skipifsilent; Check: not IsAppUpdate
Filename: "{app}\FasterDM.exe"; Flags: nowait; Check: IsAppUpdate

[Messages]
SetupWindowTitle=Faster DM суулгах
WelcomeLabel1=Faster DM суулгагчид тавтай морил
WelcomeLabel2=Том файл, видео, зураг татах Faster DM аппыг суулгана.%n%nШинэчлэхээс өмнө хуучин аппын таталтуудаа дуусгаад хаана уу.
ButtonBack=< Буцах
ButtonNext=Үргэлжлүүлэх >
ButtonCancel=Цуцлах
ButtonInstall=Суулгах
ButtonFinish=Дуусгах
FinishedHeadingLabel=Faster DM суулгаж дууслаа
FinishedLabel=Faster DM-ийг Start Menu-ээс нээх боломжтой.%n%nYouTube хэрэгслүүд багтсан. WebView2 байхгүй бол аппын анхны нээлтэд интернэт шаардлагатай.

[Code]
function CreateFile(FileName: String; Access, Share, Security, Creation, Flags, Template: LongWord): LongWord;
  external 'CreateFileW@kernel32.dll stdcall';
function CloseHandle(Handle: LongWord): Boolean;
  external 'CloseHandle@kernel32.dll stdcall';
function OpenProcess(Access: LongWord; Inherit: Boolean; ProcessId: LongWord): LongWord;
  external 'OpenProcess@kernel32.dll stdcall';
function WaitForSingleObject(Handle, Milliseconds: LongWord): LongWord;
  external 'WaitForSingleObject@kernel32.dll stdcall';

function IsAppUpdate: Boolean;
begin
  Result := ExpandConstant('{param:UPDATEPID|}') <> '';
end;

function PrepareToInstall(var NeedsRestart: Boolean): String;
var
  Handle: LongWord;
  ProcessId: Integer;
  WaitResult: LongWord;
begin
  Result := '';
  if IsAppUpdate then begin
    ProcessId := StrToIntDef(ExpandConstant('{param:UPDATEPID|0}'), 0);
    if ProcessId <= 0 then begin Result := 'Аппын процессын дугаар буруу байна.'; Exit; end;
    Handle := OpenProcess($00100000, False, ProcessId);
    if Handle <> 0 then begin
      WaitResult := WaitForSingleObject(Handle, 60000);
      CloseHandle(Handle);
      if WaitResult <> 0 then begin Result := 'Апп хаагдаж дуусаагүй байна. Дараа дахин шинэчилнэ үү.'; Exit; end;
    end;
  end;
  if FileExists(ExpandConstant('{app}\FasterDM.exe')) then begin
    Handle := CreateFile(ExpandConstant('{app}\FasterDM.exe'), $40000000, 0, 0, 3, 0, 0);
    if Handle = $FFFFFFFF then
      Result := 'Faster DM ажиллаж байна эсвэл файл түгжээтэй байна. Таталтаа дуусгаад аппыг хааж дахин оролдоно уу.';
    if Handle <> $FFFFFFFF then CloseHandle(Handle);
  end;
end;
