package download

import (
	"bufio"
	"encoding/xml"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

const (
	userAgent = "Minha Receita/0.0.1 (minhareceita.org)"

	// FederalRevenueUpdatedAt is a file that contains the date the data was
	// extracted by the Federal Revenue
	FederalRevenueUpdatedAt = "updated_at.txt"

	// FederalRevenueBaseURL is the base URL used for gathering Federal Revenue resources
	FederalRevenueBaseURL = "https://arquivos.receitafederal.gov.br/public.php/webdav"

	// Nextcloud/SERPRO+ WebDAV endpoints
	federalRevenueWebDAVURL  = FederalRevenueBaseURL
	federalRevenueShareToken = "gn672Ad4CF8N6TK"
	federalRevenueSourcePath = "/Dados/Cadastros/CNPJ"
	federalRevenueTaxesPath  = "/Dados/Obrigacoes_Acessorias"
)

var fileTimestampPattern = regexp.MustCompile(`\d{4}-\d{2}-\d{2}`)
var yearMonthPattern = regexp.MustCompile(`\d{4}-\d{2}`)
var zipFilePattern = regexp.MustCompile(`\.zip$`)

// WebDAV XML structures
type webDAVMultistatus struct {
	XMLName   xml.Name           `xml:"multistatus"`
	Responses []webDAVResponse   `xml:"response"`
}

type webDAVResponse struct {
	Href     string         `xml:"href"`
	Propstat webDAVPropstat `xml:"propstat"`
}

type webDAVPropstat struct {
	Prop webDAVProp `xml:"prop"`
}

type webDAVProp struct {
	ResourceType webDAVResourceType `xml:"resourcetype"`
	GetLastModified string          `xml:"getlastmodified"`
}

type webDAVResourceType struct {
	Collection *struct{} `xml:"collection"`
}

func get(url string) (string, error) {
	c := http.Client{}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("error creating request %s: %w", url, err)
	}
	req.Header.Set("User-Agent", userAgent)
	r, err := c.Do(req)
	if err != nil {
		return "", fmt.Errorf("error getting %s: %w", url, err)
	}
	defer func() {
		if err := r.Body.Close(); err != nil {
			slog.Warn("could not close http response", "url", url, "error", err)
		}
	}()
	if r.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%s responded with %s", url, r.Status)
	}
	b, err := io.ReadAll(r.Body)
	if err != nil {
		return "", fmt.Errorf("could not read %s response body: %w", url, err)
	}
	return string(b), nil
}

func webDAVList(path string) (*webDAVMultistatus, error) {
	url := federalRevenueWebDAVURL + path
	c := http.Client{}
	req, err := http.NewRequest("PROPFIND", url, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request %s: %w", url, err)
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Depth", "1")
	req.SetBasicAuth(federalRevenueShareToken, "")
	
	r, err := c.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error listing %s: %w", url, err)
	}
	defer func() {
		if err := r.Body.Close(); err != nil {
			slog.Warn("could not close http response", "url", url, "error", err)
		}
	}()
	if r.StatusCode != http.StatusMultiStatus {
		return nil, fmt.Errorf("%s responded with %s", url, r.Status)
	}
	
	b, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, fmt.Errorf("could not read %s response body: %w", url, err)
	}
	
	var result webDAVMultistatus
	if err := xml.Unmarshal(b, &result); err != nil {
		return nil, fmt.Errorf("could not parse WebDAV response: %w", err)
	}
	
	return &result, nil
}

func federalRevenueGetMostRecentDir(dirPath string) (string, error) {
	result, err := webDAVList(dirPath)
	if err != nil {
		return "", fmt.Errorf("error listing %s: %w", dirPath, err)
	}
	
	var months []string
	for _, resp := range result.Responses {
		// Skip the parent directory
		if resp.Href == "/public.php/webdav"+dirPath+"/" || resp.Href == "/public.php/webdav"+dirPath {
			continue
		}
		// Check if it's a directory (collection)
		if resp.Propstat.Prop.ResourceType.Collection != nil {
			// Extract the directory name from the href
			parts := strings.Split(strings.TrimSuffix(resp.Href, "/"), "/")
			if len(parts) > 0 {
				dirName := parts[len(parts)-1]
				// Check if it matches YYYY-MM pattern
				if yearMonthPattern.MatchString(dirName) {
					months = append(months, dirName)
				}
			}
		}
	}
	
	if len(months) == 0 {
		return "", fmt.Errorf("no monthly directories found in %s", dirPath)
	}
	
	slices.Sort(months)
	mostRecentMonth := months[len(months)-1]
	return dirPath + "/" + mostRecentMonth, nil
}

func buildWebDAVDownloadURL(path string) string {
	// WebDAV public download URL
	return federalRevenueWebDAVURL + path
}

func listZipFiles(dirPath string) ([]string, error) {
	result, err := webDAVList(dirPath)
	if err != nil {
		return nil, fmt.Errorf("error listing %s: %w", dirPath, err)
	}
	
	var urls []string
	for _, resp := range result.Responses {
		// Skip the parent directory
		if resp.Href == "/public.php/webdav"+dirPath+"/" || resp.Href == "/public.php/webdav"+dirPath {
			continue
		}
		// Check if it's a file (not a collection) and ends with .zip
		if resp.Propstat.Prop.ResourceType.Collection == nil {
			// Extract the file name from the href
			parts := strings.Split(resp.Href, "/")
			if len(parts) > 0 {
				fileName := parts[len(parts)-1]
				if zipFilePattern.MatchString(fileName) {
					// Build the WebDAV download URL with embedded authentication
					// Format: https://user:@host/path (empty password after colon)
					downloadURL := fmt.Sprintf("https://%s:@arquivos.receitafederal.gov.br/public.php/webdav%s/%s",
						federalRevenueShareToken, dirPath, fileName)
					urls = append(urls, downloadURL)
				}
			}
		}
	}
	
	return urls, nil
}

func taxRegimeGetURLs(dirPath string) ([]string, error) {
	// Tax regime data might be in a subdirectory or not available yet
	// Try to list and find relevant files
	urls, err := listZipFiles(dirPath)
	if err != nil {
		// Tax regime data might not be available yet in the new structure
		slog.Warn("could not get tax regime data", "path", dirPath, "error", err)
		return []string{}, nil
	}
	return urls, nil
}

func federalRevenueGetURLs(_ string) ([]string, error) {
	// Get the most recent month directory
	mostRecentDir, err := federalRevenueGetMostRecentDir(federalRevenueSourcePath)
	if err != nil {
		return nil, fmt.Errorf("could not get most recent directory: %w", err)
	}
	
	// List all ZIP files in the most recent month directory
	urls, err := listZipFiles(mostRecentDir)
	if err != nil {
		return nil, fmt.Errorf("error listing files in %s: %w", mostRecentDir, err)
	}
	
	// Try to get tax regime files (may not exist in new structure)
	ts, err := taxRegimeGetURLs(federalRevenueTaxesPath)
	if err != nil {
		// Don't fail if tax regime is not available
		slog.Warn("could not get tax regime URLs", "error", err)
	} else {
		urls = append(urls, ts...)
	}
	
	return urls, nil
}

func saveUpdatedAt(dir string) (err error) { // using named return so we can set it in the defer call
	mostRecentDir, err := federalRevenueGetMostRecentDir(federalRevenueSourcePath)
	if err != nil {
		return fmt.Errorf("error getting most recent source directory: %w", err)
	}
	
	// Extract YYYY-MM from the directory path (e.g., "/Dados/Cadastros/CNPJ/2024-11")
	parts := strings.Split(mostRecentDir, "/")
	if len(parts) == 0 {
		return fmt.Errorf("could not extract date from directory path: %s", mostRecentDir)
	}
	
	lastPart := parts[len(parts)-1]
	if len(lastPart) == 7 && lastPart[4] == '-' { // Format: YYYY-MM
		// Convert YYYY-MM to YYYY-MM-01
		d := lastPart + "-01"
		pth := filepath.Join(dir, FederalRevenueUpdatedAt)
		f, err := os.Create(pth)
		if err != nil {
			return fmt.Errorf("error creating %s: %w", pth, err)
		}
		defer func() {
			if e := f.Close(); e != nil && err == nil {
				err = fmt.Errorf("could not close %s: %w", pth, e)
			}
		}()
		w := bufio.NewWriter(f)
		_, err = w.WriteString(d)
		if err != nil {
			return fmt.Errorf("error writing %s: %w", pth, err)
		}
		return w.Flush()
	}
	
	return fmt.Errorf("could not extract valid date from directory name: %s", lastPart)
}
