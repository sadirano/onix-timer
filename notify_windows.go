//go:build windows

package main

import (
	"os"
	"os/exec"
	"syscall"
)

func showToast(message string) {
	// Message is delivered via environment variable so it never enters the
	// PowerShell command string — prevents injection through crafted timer names.
	const ps = `
[Windows.UI.Notifications.ToastNotificationManager, Windows.UI.Notifications, ContentType=WindowsRuntime] | Out-Null
[Windows.Data.Xml.Dom.XmlDocument, Windows.Data.Xml.Dom, ContentType=WindowsRuntime] | Out-Null
$template = [Windows.UI.Notifications.ToastNotificationManager]::GetTemplateContent(
    [Windows.UI.Notifications.ToastTemplateType]::ToastText01)
$template.GetElementsByTagName('text')[0].AppendChild(
    $template.CreateTextNode($env:ONIX_TOAST_MSG)) | Out-Null
$toast = [Windows.UI.Notifications.ToastNotification]::new($template)
[Windows.UI.Notifications.ToastNotificationManager]::CreateToastNotifier('onix-timer').Show($toast)
`
	cmd := exec.Command("powershell.exe",
		"-NoProfile", "-NonInteractive", "-WindowStyle", "Hidden", "-Command", ps)
	cmd.Env = append(os.Environ(), "ONIX_TOAST_MSG="+message)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if err := cmd.Run(); err != nil {
		// Fallback: msg.exe dialog (always present on Windows)
		fallback := exec.Command("msg.exe", "*", "/time:30", message)
		fallback.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
		_ = fallback.Start()
	}
}

func runOnDone(command string) {
	if command == "" {
		return
	}
	cmd := exec.Command("cmd.exe", "/C", command)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}
	_ = cmd.Start()
}
