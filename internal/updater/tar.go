package updater

import (
	"archive/tar"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// extractFromTar reads a tar stream and extracts the "rad" binary to a temp file.
func extractFromTar(r io.Reader) (string, error) {
	tr := tar.NewReader(r)

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("read tar: %w", err)
		}

		// Look for the rad binary (could be at root or in a subdirectory)
		if filepath.Base(hdr.Name) == "rad" && hdr.Typeflag == tar.TypeReg {
			tmp, err := os.CreateTemp("", "rad-bin-*")
			if err != nil {
				return "", err
			}

			if _, err := io.Copy(tmp, tr); err != nil {
				tmp.Close()
				os.Remove(tmp.Name())
				return "", err
			}
			tmp.Close()

			// Make executable
			os.Chmod(tmp.Name(), 0755)
			return tmp.Name(), nil
		}
	}

	return "", fmt.Errorf("rad binary not found in tarball")
}
