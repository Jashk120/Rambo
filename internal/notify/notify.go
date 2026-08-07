package notify

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

func Send(title, body string) {
	// try notify-send (GNOME, KDE, most DEs)
	if err := exec.Command("notify-send", title, body).Run(); err == nil {
		return
	}

	// try zenity (fallback GUI dialog)
	if err := exec.Command("zenity", "--info", "--title", title, "--text", body).Run(); err == nil {
		return
	}

	// try kdialog (KDE specific)
	if err := exec.Command("kdialog", "--title", title, "--passivepopup", body, "5").Run(); err == nil {
		return
	}

	// last resort — write to a log file they can check
	logToFile(title, body)
}

func logToFile(title, body string) {
	home, _ := os.UserHomeDir()
	path := filepath.Join(home, ".local", "share", "rambo", "rambo.log")
	os.MkdirAll(filepath.Dir(path), 0755)

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Printf("[rambo] %s: %s\n", title, body)
		return
	}
	defer f.Close()

	fmt.Fprintf(f, "[%s] %s: %s\n", time.Now().Format("2006-01-02 15:04:05"), title, body)
}
