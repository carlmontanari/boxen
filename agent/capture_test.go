package agent

import (
	"encoding/base64"
	"strings"
	"testing"

	boxenassets "github.com/carlmontanari/boxen/assets"
	boxenprofile "github.com/carlmontanari/boxen/profile"
)

func TestExtractCapture(t *testing.T) {
	markers := boxenprofile.StepCapture{
		Start: boxenprofile.Contains{ContainsPattern: `(?m)^BEGIN$`},
		End:   boxenprofile.Contains{ContainsPattern: `(?m)^END$`},
	}

	prompt := boxenprofile.StepCapture{
		End: boxenprofile.Contains{ContainsPattern: `(?m)^r1#`},
	}

	for _, test := range []struct {
		name    string
		capture boxenprofile.StepCapture
		raw     string
		want    string
		done    bool
	}{
		{
			name:    "markers with console line endings",
			capture: markers,
			raw:     "echo stuff\r\nBEGIN\r\nline 1\r\nline 2\r\nEND\r\nr1# ",
			want:    "line 1\nline 2\n",
			done:    true,
		},
		{
			name:    "start marker line not complete",
			capture: markers,
			raw:     "BEGIN",
		},
		{
			name:    "end marker not seen yet",
			capture: markers,
			raw:     "BEGIN\r\nline 1\r\n",
		},
		{
			name:    "end marker before start is ignored",
			capture: markers,
			raw:     "END\r\nBEGIN\r\nline 1\r\nEND\r\n",
			want:    "line 1\n",
			done:    true,
		},
		{
			name:    "prompt delimited output",
			capture: prompt,
			raw:     "\r\nhostname r1\r\n!\r\nend\r\nr1#",
			want:    "hostname r1\n!\nend\n",
			done:    true,
		},
		{
			name:    "nul bytes are dropped",
			capture: prompt,
			raw:     "\x00\r\nhostname r1\x00\r\nr1#",
			want:    "hostname r1\n",
			done:    true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, _, done, err := extractCapture([]byte(test.raw), &test.capture)
			if err != nil {
				t.Fatal(err)
			}

			if done != test.done || got != test.want {
				t.Fatalf("got (%q, %v), want (%q, %v)", got, done, test.want, test.done)
			}
		})
	}
}

func TestExtractCaptureBase64(t *testing.T) {
	content := `{"DEVICE_METADATA": {"localhost": {"hostname": "leaf1"}}}` + "\n"
	encoded := base64.StdEncoding.EncodeToString([]byte(content))

	capture := boxenprofile.StepCapture{
		Start:  boxenprofile.Contains{ContainsPattern: `(?m)^BEGIN$`},
		End:    boxenprofile.Contains{ContainsPattern: `(?m)^END$`},
		Decode: boxenprofile.CaptureDecodeBase64,
	}

	raw := "BEGIN\r\n" + encoded[:20] + "\r\n" + encoded[20:] + "\r\nEND\r\n"

	got, _, done, err := extractCapture([]byte(raw), &capture)
	if err != nil || !done || got != content {
		t.Fatalf("got (%q, %v, %v), want %q", got, done, err, content)
	}

	got, _, done, err = extractCapture([]byte("BEGIN\r\nnot base64!\r\nEND\r\n"), &capture)
	if err == nil {
		t.Fatalf("expected a decode error, got (%q, %v)", got, done)
	}
}

func loadEmbeddedProfile(t *testing.T, name string) *boxenprofile.Profile {
	t.Helper()

	b, err := boxenassets.Assets.ReadFile("profiles/" + name + ".yaml")
	if err != nil {
		t.Fatal(err)
	}

	p, err := boxenprofile.Load(b)
	if err != nil {
		t.Fatal(err)
	}

	return p
}

func saveCaptureStep(t *testing.T, p *boxenprofile.Profile) *boxenprofile.StepCapture {
	t.Helper()

	for idx := range p.Run.SaveProcess {
		if p.Run.SaveProcess[idx].Type == boxenprofile.StepTypeCapture {
			return &p.Run.SaveProcess[idx].Capture
		}
	}

	t.Fatal("profile save process has no capture step")

	return nil
}

func TestCumulusSaveCapture(t *testing.T) {
	capture := saveCaptureStep(t, loadEmbeddedProfile(t, "nvidia_cumulusvx"))

	// the console echoes the command (with bracketed paste sequences) before its output
	raw := "\x1b[?2004hcumulus@leaf1:mgmt:~$ " + capture.Command + "\r\n\x1b[?2004l\r" +
		"BOXEN_BEGIN\r\n" +
		"nv set interface eth0 ipv4 address 172.20.20.2/24\r\n" +
		"nv set system hostname leaf1\r\n" +
		"BOXEN_END\r\n" +
		"\x1b[?2004hcumulus@leaf1:mgmt:~$ "

	got, _, done, err := extractCapture([]byte(raw), capture)
	if err != nil {
		t.Fatal(err)
	}

	want := "nv set interface eth0 ipv4 address 172.20.20.2/24\nnv set system hostname leaf1\n"
	if !done || got != want {
		t.Fatalf("got (%q, %v), want %q", got, done, want)
	}
}

func TestSonicSaveCapture(t *testing.T) {
	capture := saveCaptureStep(t, loadEmbeddedProfile(t, "community_sonic"))

	if strings.Contains(capture.Command, "BOXEN_BEGIN") ||
		strings.Contains(capture.Command, "BOXEN_END") {
		t.Fatal("the echoed command must not contain the literal capture markers")
	}

	content := `{"DEVICE_METADATA": {"localhost": {"hostname": "leaf1"}}}` + "\n"
	encoded := base64.StdEncoding.EncodeToString([]byte(content))

	raw := "admin@leaf1:~$ " + capture.Command + "\r\n" +
		"BOXEN_BEGIN\r\n" + encoded + "\r\nBOXEN_END\r\nadmin@leaf1:~$ "

	got, _, done, err := extractCapture([]byte(raw), capture)
	if err != nil || !done || got != content {
		t.Fatalf("got (%q, %v, %v), want %q", got, done, err, content)
	}
}

func TestCiscoSaveCapture(t *testing.T) {
	for _, test := range []struct {
		profile string
		raw     string
		want    string
	}{
		{
			profile: "cisco_csr1000v",
			raw: "show running-config brief | exclude " +
				"^ certificate|^platform console|^diagnostic bootup|^license udi\r\n" +
				"Building configuration...\r\n\r\n" +
				"Current configuration : 1234 bytes\r\n!\r\nhostname csr1\r\n!\r\nend\r\n\r\ncsr1#",
			want: "!\nhostname csr1\n!\nend\n\n",
		},
	} {
		t.Run(test.profile, func(t *testing.T) {
			capture := saveCaptureStep(t, loadEmbeddedProfile(t, test.profile))

			// the echoed command is consumed before the output is read
			raw := strings.TrimPrefix(test.raw, capture.Command)

			got, _, done, err := extractCapture([]byte(raw), capture)
			if err != nil || !done || got != test.want {
				t.Fatalf("got (%q, %v, %v), want %q", got, done, err, test.want)
			}
		})
	}
}
