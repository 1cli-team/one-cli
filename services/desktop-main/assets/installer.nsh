; Reuse Tauri's per-user uninstall key and original installation directory.
; No existing data or installation is removed before the new files are ready.
!macro customInit
  ReadRegStr $0 HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\One Desktop" "MainBinaryName"
  ${If} $0 != ""
    ReadRegStr $1 HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\One Desktop" "InstallLocation"
    StrCpy $2 $1 1
    ${If} $2 == '$\"'
      StrCpy $1 $1 -1 1
    ${EndIf}
    ${If} $1 != ""
      StrCpy $INSTDIR $1
    ${EndIf}
  ${EndIf}
!macroend

!macro customInstall
  ; The old uninstaller must not remain usable after replacing its registry entry.
  Delete "$INSTDIR\uninstall.exe"
  DeleteRegValue HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\One Desktop" "MainBinaryName"
!macroend
