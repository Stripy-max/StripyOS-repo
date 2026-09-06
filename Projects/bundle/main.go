package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func printHelp() {
	fmt.Println("StripyOS Bundle Manager (str)")
	fmt.Println("Usage:")
	fmt.Println("  str -Sl <file.tar.xz>    Install a local bundle package")
	fmt.Println("  str -R <pkg_name>        Remove/uninstall a package completely")
	fmt.Println("  str -L                   List all installed packages")
	fmt.Println("  str -hlp                 Show this help message")
}

func updateDesktopDatabase(targetAppsDir string) {
	// إجبار اللانشر على إنعاش قاعدة البيانات وتحديث قائمة التطبيقات
	exec.Command("update-desktop-database", targetAppsDir).Run()
}

func setupDesktopShortcut(pkgName, installDir string) {
	appsDir := filepath.Join(installDir, "share", "applications")
	files, err := os.ReadDir(appsDir)
	if err != nil {
		return // لا يوجد مجلد اختصارات داخل الحزمة
	}

	homeDir, _ := os.UserHomeDir()
	targetAppsDir := filepath.Join(homeDir, ".local", "share", "applications")
	os.MkdirAll(targetAppsDir, 0755)

	for _, f := range files {
		if strings.HasSuffix(f.Name(), ".desktop") {
			srcFile := filepath.Join(appsDir, f.Name())

			// إصلاح اسم الملف المنسوخ لمنع التكرار (str-gimp.desktop بدلاً من str-gimp-gimp.desktop)
			dstFile := filepath.Join(targetAppsDir, fmt.Sprintf("str-%s", f.Name()))

			processDesktopFile(srcFile, dstFile, pkgName, installDir)
			fmt.Printf("Desktop shortcut integrated: %s\n", filepath.Base(dstFile))
		}
	}

	updateDesktopDatabase(targetAppsDir)
}

func processDesktopFile(srcFile, dstFile, pkgName, installDir string) {
	inputFile, err := os.Open(srcFile)
	if err != nil {
		return
	}
	defer inputFile.Close()

	outputFile, err := os.Create(dstFile)
	if err != nil {
		return
	}
	defer outputFile.Close()

	scanner := bufio.NewScanner(inputFile)
	writer := bufio.NewWriter(outputFile)

	execPath := filepath.Join(installDir, "bin", pkgName)
	iconPath := filepath.Join(installDir, "share", "icons")

	for scanner.Scan() {
		line := scanner.Text()

		// 1. حذف سطر TryExec لأنه يمنع ظهور التطبيق في اللانشر
		if strings.HasPrefix(line, "TryExec=") {
			continue
		}

		// 2. تصحيح مسار التنفيذ المحلي
		if strings.HasPrefix(line, "Exec=") {
			line = fmt.Sprintf("Exec=%s", execPath)
		} else if strings.HasPrefix(line, "Icon=") {
			// 3. ربط مسار الأيقونة المباشر
			foundIcon := findIconPath(iconPath, pkgName)
			if foundIcon != "" {
				line = fmt.Sprintf("Icon=%s", foundIcon)
			}
		}
		writer.WriteString(line + "\n")
	}
	writer.Flush()
}

func findIconPath(baseIconsDir, pkgName string) string {
	var iconPath string
	filepath.Walk(baseIconsDir, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() && (strings.HasSuffix(path, ".png") || strings.HasSuffix(path, ".svg")) {
			if strings.Contains(strings.ToLower(info.Name()), strings.ToLower(pkgName)) {
				iconPath = path
				return filepath.SkipAll
			}
		}
		return nil
	})
	return iconPath
}

func removeDesktopShortcut(pkgName string) {
	homeDir, _ := os.UserHomeDir()
	targetAppsDir := filepath.Join(homeDir, ".local", "share", "applications")

	files, err := os.ReadDir(targetAppsDir)
	if err != nil {
		return
	}

	prefix := fmt.Sprintf("str-%s", pkgName)
	for _, f := range files {
		if strings.HasPrefix(f.Name(), prefix) {
			os.Remove(filepath.Join(targetAppsDir, f.Name()))
			fmt.Printf("Removed shortcut: %s\n", f.Name())
		}
	}

	updateDesktopDatabase(targetAppsDir)
}

func main() {
	if len(os.Args) < 2 {
		printHelp()
		return
	}

	command := os.Args[1]
	homeDir, _ := os.UserHomeDir()
	bundlesDir := filepath.Join(homeDir, ".local", "share", "bundles")

	switch command {
		case "-hlp":
			printHelp()

		case "-L":
			fmt.Println("Installed bundles:")
			files, err := os.ReadDir(bundlesDir)
			if err != nil {
				fmt.Println("No bundles found or directory does not exist.")
				return
			}
			for _, f := range files {
				if f.IsDir() {
					fmt.Printf(" - %s\n", f.Name())
				}
			}

		case "-Sl":
			if len(os.Args) < 3 {
				fmt.Println("Error: Please specify the local tar.xz file path.")
				return
			}
			archivePath := os.Args[2]
			pkgName := filepath.Base(archivePath)
			if len(archivePath) > 7 && archivePath[len(archivePath)-7:] == ".tar.xz" {
				pkgName = filepath.Base(archivePath[:len(archivePath)-7])
			}

			installDir := filepath.Join(bundlesDir, pkgName)
			os.MkdirAll(installDir, 0755)

			fmt.Printf("Installing %s locally...\n", pkgName)
			cmd := exec.Command("tar", "--no-same-owner", "-xf", archivePath, "-C", installDir)
			if output, err := cmd.CombinedOutput(); err != nil {
				fmt.Printf("Error extracting: %v\nDetails: %s\n", err, string(output))
				return
			}

			setupDesktopShortcut(pkgName, installDir)
			fmt.Printf("Successfully installed %s!\n", pkgName)

		case "-R":
			if len(os.Args) < 3 {
				fmt.Println("Error: Please specify the package name to remove.")
				return
			}
			pkgName := os.Args[2]
			targetDir := filepath.Join(bundlesDir, pkgName)

			if _, err := os.Stat(targetDir); os.IsNotExist(err) {
				fmt.Printf("Package '%s' is not installed.\n", pkgName)
				return
			}

			removeDesktopShortcut(pkgName)
			err := os.RemoveAll(targetDir)
			if err != nil {
				fmt.Printf("Error removing package: %v\n", err)
				return
			}
			fmt.Printf("Successfully removed package '%s'.\n", pkgName)

		default:
			fmt.Printf("Unknown command: %s\n", command)
			printHelp()
	}
}
