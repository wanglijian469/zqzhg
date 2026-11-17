// Standalone PDF connectivity check: no business database and no public upload.
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	pdfapi "github.com/pdfcpu/pdfcpu/pkg/api"
	pdfmodel "github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	"io"
	"net/http/httptrace"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"zqzt/exhibition/internal/bailian"
)

func main() {
	p := flag.String("pdf", "", "private PDF path")
	tempRoot := flag.String("temp-dir", ".local/bailian-preflight-tmp", "private temporary directory")
	probePage := flag.Int("probe-page", 0, "send a compact rendered real source page as PDF; original is unchanged")
	textOnly := flag.Bool("text", false, "diagnose text connectivity without a PDF")
	trace := flag.Bool("trace", false, "print network phase names, never headers/payloads")
	attemptsLimit := flag.Int("attempts", 3, "maximum attempts (1-3)")
	review := flag.Bool("review", false, "test review model")
	explicitModel := flag.String("model", "", "explicit model override, e.g. qwen3.8-max for a manually chosen advanced check")
	out := flag.String("output", "", "private JSON report path")
	flag.Parse()
	if *p == "" && !*textOnly {
		fmt.Fprintln(os.Stderr, "--pdf required")
		os.Exit(2)
	}
	data, e := os.ReadFile(*p)
	if e != nil && !*textOnly {
		fmt.Fprintln(os.Stderr, "pdf_read_failed")
		os.Exit(1)
	}
	sourceHash := fmt.Sprintf("%x", sha256.Sum256(data))
	if *probePage > 0 {
		if err := os.MkdirAll(*tempRoot, 0700); err != nil {
			fmt.Fprintln(os.Stderr, "probe_temp_root_failed")
			os.Exit(1)
		}
		dir, err := os.MkdirTemp(*tempRoot, "bailian-pdf-probe-")
		if err != nil {
			fmt.Fprintln(os.Stderr, "probe_temp_failed")
			os.Exit(1)
		}
		defer os.RemoveAll(dir)
		tool := os.Getenv("PDFTOPPM_PATH")
		if tool == "" {
			tool = "pdftoppm"
		}
		sourceFile := filepath.Join(dir, "source.pdf")
		if err = os.WriteFile(sourceFile, data, 0600); err != nil {
			fmt.Fprintln(os.Stderr, "probe_source_failed")
			os.Exit(1)
		}
		prefix := filepath.Join(dir, "page")
		cmd := exec.Command(tool, "-f", strconv.Itoa(*probePage), "-l", strconv.Itoa(*probePage), "-singlefile", "-scale-to", "1400", "-jpeg", sourceFile, prefix)
		if err = cmd.Run(); err != nil {
			fmt.Fprintln(os.Stderr, "probe_render_failed")
			os.Exit(1)
		}
		img, err := os.ReadFile(prefix + ".jpg")
		if err != nil {
			fmt.Fprintln(os.Stderr, "probe_read_failed")
			os.Exit(1)
		}
		var out bytes.Buffer
		pdfmodel.ConfigPath = "disable"
		if err = pdfapi.ImportImages(context.Background(), nil, &out, []io.Reader{bytes.NewReader(img)}, nil, pdfmodel.NewDefaultConfiguration()); err != nil {
			fmt.Fprintln(os.Stderr, "probe_pdf_failed")
			os.Exit(1)
		}
		data = out.Bytes()
	}
	cfg := bailian.FromEnv()
	if *attemptsLimit < 1 || *attemptsLimit > 3 {
		fmt.Fprintln(os.Stderr, "invalid attempts")
		os.Exit(2)
	}
	cfg.Retries = *attemptsLimit - 1
	client := bailian.Client{Config: cfg}
	model := cfg.Model
	if *review {
		model = cfg.ReviewModel
	}
	if *explicitModel != "" {
		model = *explicitModel
	}
	attempts := []bailian.Attempt{}
	parts := []bailian.Part{bailian.PDF(filepath.Base(*p), data)}
	if *textOnly {
		parts = []bailian.Part{{Type: "text", Text: "Return {\"ok\":true}"}}
	}
	ctx := context.Background()
	if *trace {
		ctx = httptrace.WithClientTrace(ctx, &httptrace.ClientTrace{WroteHeaders: func() { fmt.Fprintln(os.Stderr, "phase=request_headers_sent") }, WroteRequest: func(info httptrace.WroteRequestInfo) {
			fmt.Fprintf(os.Stderr, "phase=request_body_sent success=%t\n", info.Err == nil)
		}, GotFirstResponseByte: func() { fmt.Fprintln(os.Stderr, "phase=response_started") }})
	}
	r, e := client.Complete(ctx, model, "Extract only company name and a short document summary with physical PDF source_pages. Never invent products or specifications.", parts, func(a bailian.Attempt) error {
		attempts = append(attempts, a)
		fmt.Fprintf(os.Stderr, "attempt=%d status=%d error=%s duration_ms=%d\n", a.Number, a.Status, a.Error, a.DurationMS)
		return nil
	})
	report := map[string]any{"source_sha256": sourceHash, "probe_page": *probePage, "submitted_bytes": len(data), "model": model, "attempts": attempts, "success": e == nil}
	if e != nil {
		report["error"] = e.Error()
	} else {
		report["result"] = json.RawMessage(r.Text)
	}
	b, _ := json.MarshalIndent(report, "", "  ")
	if *out != "" {
		if err := os.WriteFile(*out, b, 0600); err != nil {
			fmt.Fprintln(os.Stderr, "report_write_failed")
			os.Exit(1)
		}
	}
	fmt.Println(string(b))
	if e != nil {
		os.Exit(1)
	}
}
