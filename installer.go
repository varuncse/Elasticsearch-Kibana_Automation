package main

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

func extractZIP(zipPath, destDir string) error {
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer r.Close()
	for _, f := range r.File {
		if strings.HasPrefix(f.Name, "__MACOSX/") {
			continue
		}
		fpath := filepath.Join(destDir, f.Name)
		if !strings.HasPrefix(fpath, filepath.Clean(destDir)+string(os.PathSeparator)) {
			return fmt.Errorf("illegal file path: %s", fpath)
		}
		if f.FileInfo().IsDir() {
			os.MkdirAll(fpath, os.ModePerm)
			continue
		}
		if err := os.MkdirAll(filepath.Dir(fpath), os.ModePerm); err != nil {
			return err
		}
		outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		_, err = io.Copy(outFile, rc)
		outFile.Close()
		rc.Close()
		if err != nil {
			return err
		}
	}
	return nil
}
func extractTGZ(tgzPath, destDir string) error {
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	file, err := os.Open(tgzPath)
	if err != nil {
		return err
	}
	defer file.Close()
	gzr, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gzr.Close()
	tr := tar.NewReader(gzr)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		target := filepath.Join(destDir, header.Name)
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, os.FileMode(header.Mode)); err != nil {
				return err
			}
		case tar.TypeReg:
			outFile, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, os.FileMode(header.Mode))
			if err != nil {
				return err
			}
			if _, err := io.Copy(outFile, tr); err != nil {
				outFile.Close()
				return err
			}
			outFile.Close()
		case tar.TypeSymlink:
			if err := os.Symlink(header.Linkname, target); err != nil {
				return err
			}
		}
	}
	return nil
}
func startProcessE(logviewd, directory string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("bash", "-c", fmt.Sprintf("export JAVA_HOME=%s && cd %s/%s && %s", os.Getenv("JAVA_HOME"), logviewd, directory, `./bin/elasticsearch`))
	case "windows":
		cmd = exec.Command("cmd", "/c", fmt.Sprintf("cd %s/%s && %s", logviewd, directory, `.\bin\elasticsearch.bat`))
	case "linux":
		cmd = exec.Command("sh", "-c", fmt.Sprintf("cd %s/%s && %s", logviewd, directory, `./bin/elasticsearch`))
	default:
		return fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}
	cmd.Start()
	fmt.Println("Waiting 60 seconds for Elasticsearch to start...")
	time.Sleep(60 * time.Second)
	fmt.Println("Starting Kibana...")
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("bash", "-c", fmt.Sprintf("export JAVA_HOME=%s && cd %s/%s && %s", os.Getenv("JAVA_HOME"), logviewd, `kibana-7.17.20-darwin-x86_64`, `./bin/kibana`))
	case "windows":
		cmd = exec.Command("cmd", "/c", fmt.Sprintf("cd %s/%s && %s", logviewd, `kibana-7.17.20-windows-x86_64`, `.\bin\kibana.bat`))
	case "linux":
		cmd = exec.Command("sh", "-c", fmt.Sprintf("cd %s/%s && %s", logviewd, `kibana-7.17.20-linux-x86_64`, `./bin/kibana`))
	default:
		return fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}
	return cmd.Start()
}

func main() {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Println(err)
		return
	}
	logViewerDir := filepath.Join(homeDir, "LogViewer")
	if err := os.MkdirAll(logViewerDir, 0755); err != nil {
		fmt.Printf("Failed to create logviewer directory: %v\n", err)
		return
	}
	exeDir, err := os.Executable()
	if err != nil {
		fmt.Printf("Error getting executable directory: %v\n", err)
		return
	}
	sourceFilePath := filepath.Dir(exeDir)
	var elasticP, kibanaP string
	switch runtime.GOOS {
	case "darwin":
		jdkP := filepath.Join(sourceFilePath, "jdk-17_macos-x64_bin.tar.gz")
		elasticP = filepath.Join(sourceFilePath, "elasticsearch-7.17.20-darwin-x86_64.tar.gz")
		kibanaP = filepath.Join(sourceFilePath, "kibana-7.17.20-darwin-x86_64.tar.gz")
		if err := extractTGZ(jdkP, logViewerDir); err != nil {
			fmt.Printf("Failed to extract Java: %v\n", err)
		}
		os.Setenv("JAVA_HOME", "~/LogViewer/jdk-17.0.11.jdk/Contents/Home")
	case "linux":
		var cmd *exec.Cmd
		elasticP = filepath.Join(sourceFilePath, "elasticsearch-7.17.20-linux-x86_64.tar.gz")
		cmd = exec.Command("tar", "-xzf", "kibana-7.17.20-linux-x86_64.tar.gz", "-C", homeDir+"/LogViewer")
		err := cmd.Run()
		fmt.Println(err)
	case "windows":
		var cmd *exec.Cmd
		msiFileName := "jdk-17_windows-x64_bin.msi"
		msiFilePath := filepath.Join(sourceFilePath, msiFileName)
		cmd = exec.Command("cmd", "/c", "start", "/wait", msiFilePath)
		cmd.Run()
		currentPath := os.Getenv("PATH")
		javaPath := "C:\\Program Files\\Common Files\\Oracle\\Java\\javapath"
		newPath := fmt.Sprintf("%s;%s", currentPath, javaPath)
		err = os.Setenv("PATH", newPath)
		if err != nil {
			fmt.Println("Failed to set PATH environment variable:", err)
		}
		elasticP = filepath.Join(sourceFilePath, "elasticsearch-7.17.20-windows-x86_64.zip")
		kibanaP = filepath.Join(sourceFilePath, "kibana-7.17.20-windows-x86_64.zip")
	default:
		fmt.Println("Unsupported operating system:")
		return
	}
	if err := extractTGZ(elasticP, logViewerDir); err != nil {
		fmt.Printf("Failed to extract Elasticsearch: %v\n", err)
		return
	}
	if runtime.GOOS != "linux" {
		if err := extractTGZ(kibanaP, logViewerDir); err != nil {
			fmt.Printf("Failed to extract Kibana: %v\n", err)
			return
		}
	}
	fmt.Println("Starting Elasticsearch...")
	if err := startProcessE(logViewerDir, "elasticsearch-7.17.20"); err != nil {
		fmt.Println("Failed to start Elasticsearch:", err)
		return
	}
	if runtime.GOOS == "windows" {
		fmt.Println("Waiting 4 mins for Kibana to start...")
		time.Sleep(240 * time.Second)
	} else {
		fmt.Println("Waiting 90 seconds for Kibana to start...")
		time.Sleep(90 * time.Second)
	}
	fmt.Println("Opening localhost:5601 in default browser...")
	if runtime.GOOS == "windows" {
		cmd := exec.Command("cmd", "/c", "start", "http://localhost:5601")
		cmd.Run()
	}
	if runtime.GOOS != "windows" {
		if err := exec.Command("open", "http://localhost:5601").Start(); err != nil {
			fmt.Println("Failed to open Kibana in browser:", err)
		}
	}
}
