package qrcodeapi

import (
	"context"
	"image"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/whitekid/goxp/request"

	"qrcodeapi/pkg/qrcode"
)

func TestText(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	ts := newTestServer(ctx, newAPIv1())

	type args struct {
		width     int
		height    int
		imageType string
	}
	tests := [...]struct {
		name            string
		args            args
		wantW, wantH    int
		wantContentType string
		wantImage       string
		wantErr         bool
	}{
		{"default", args{0, 0, ""}, 200, 200, "image/png", "png", false},
		{"default", args{0, 0, "png"}, 200, 200, "image/png", "png", false},
		{"default", args{0, 0, "jpg"}, 200, 200, "image/jpeg", "jpeg", false},
		{"default", args{0, 0, "gif"}, 200, 200, "image/gif", "gif", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := request.Get("%s/qrcode", ts.URL).Query("content", "hello world")

			if tt.args.width > 0 {
				req = req.Query("w", strconv.FormatInt(int64(tt.args.width), 10))
			}

			if tt.args.height > 0 {
				req = req.Query("h", strconv.FormatInt(int64(tt.args.height), 10))
			}

			if tt.args.imageType != "" {
				req = req.Query("t", tt.args.imageType)
			}

			resp, err := req.Do(ctx)
			require.Falsef(t, (err != nil) != tt.wantErr, `qrcode request failed`, `error = %v, wantErr = %v`, err, tt.wantErr)
			require.NoError(t, err)
			require.Truef(t, resp.Success(), "failed with %d", resp.StatusCode)

			require.Equal(t, tt.wantContentType, resp.Header.Get(request.HeaderContentType))

			defer resp.Body.Close()
			img, s, err := image.Decode(resp.Body)

			require.NoError(t, err)
			require.Equal(t, image.Point{tt.wantW, tt.wantH}, img.Bounds().Size(), "size not equals")
			require.Equal(t, tt.wantImage, s)

			got, err := qrcode.Decode(img)
			require.NoError(t, err)
			require.Equal(t, "hello world", got)
		})
	}
}

func TestURL(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	ts := newTestServer(ctx, newAPIv1())

	resp, err := request.Get("%s/qrcode", ts.URL).
		Query("url", "google.com").Do(ctx)
	require.NoError(t, err)
	require.True(t, resp.Success())

	require.Equal(t, "image/png", resp.Header.Get(request.HeaderContentType))

	defer resp.Body.Close()
	img, _, err := image.Decode(resp.Body)
	require.NoError(t, err)
	got, err := qrcode.Decode(img)
	require.NoError(t, err)
	require.Equal(t, "URLTO:google.com", got)
}

func TestWifi(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	ts := newTestServer(ctx, newAPIv1())

	type args struct {
		query    map[string]string
		wantCode string
	}
	tests := [...]struct {
		name       string
		arg        args
		wantErr    bool
		wantStatus int
	}{
		{"empty ssid", args{map[string]string{
			"ssid": "myssid",
		}, ""}, false, http.StatusBadRequest},
		{"valid", args{map[string]string{
			"ssid":   "myssid",
			"auth":   "WPA",
			"pass":   "mypassword",
			"hidden": "true",
			"eap":    "TTLS",
			"anon":   "anon_id",
			"ident":  "my_ident",
			"ph2":    "MSCHAPV2",
		}, "WIFI:S:myssid;T:WPA;P:mypassword;H:true;E:TTLS;A:anon_id;I:my_ident;PH2:MSCHAPV2;;"}, false, http.StatusOK},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := request.Get("%s/qrcode", ts.URL).Queries(tt.arg.query).Do(ctx)
			if (err != nil) != tt.wantErr {
				require.Failf(t, `wifi request failed`, `error = %v, wantErr = %v`, err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}

			require.Equalf(t, tt.wantStatus, resp.StatusCode, "status=%d, wantCode=%d", resp.StatusCode, tt.wantStatus)
			if !resp.Success() {
				return
			}

			require.Truef(t, resp.Success(), "failed with status %s", resp.Status)
			require.Equal(t, "image/png", resp.Header.Get(request.HeaderContentType))

			defer resp.Body.Close()
			img, _, err := image.Decode(resp.Body)
			require.NoError(t, err)

			decoded, err := qrcode.Decode(img)
			require.NoError(t, err)
			require.Equal(t, tt.arg.wantCode, decoded)
		})
	}
}

func TestContact(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	ts := newTestServer(ctx, newAPIv1())

	resp, err := request.Get("%s/contact", ts.URL).
		Query("name[first]", "firstname").
		Query("name[last]", "lastname").
		Do(ctx)
	require.NoError(t, err)
	require.True(t, resp.Success())

	require.Equal(t, "image/png", resp.Header.Get(request.HeaderContentType))
}

func TestContactVCF(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	ts := newTestServer(ctx, newAPIv1())

	content := `BEGIN:VCARD
VERSION:4.0
N:lastname;firstname;;;
END:VCARD`

	resp, err := request.Post("%s/vcard", ts.URL).
		ContentType(mimeVCard).
		Body(strings.NewReader(content)).
		Do(ctx)
	require.NoError(t, err)
	require.True(t, resp.Success(), "failed with status %d: %s", resp.StatusCode, resp.Status)

	require.Equal(t, "image/png", resp.Header.Get(request.HeaderContentType))
	defer resp.Body.Close()
	img, _, err := image.Decode(resp.Body)
	require.NoError(t, err)
	got, err := qrcode.Decode(img)
	require.NoError(t, err)
	require.Equal(t, strings.ReplaceAll(content, "\n", "\r\n"), got)
}

// VEvent??? QR ??????????????? ?????????
func TestVEvent(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	ts := newTestServer(ctx, newAPIv1())

	content := `BEGIN:VEVENT
SUMMARY:Summer+Vacation!
DTSTART:20180601T070000Z
DTEND:20180831T070000Z
END:VEVENT`

	resp, err := request.Post("%s/vevent", ts.URL).
		ContentType(mimeVEvent).
		Body(strings.NewReader(content)).
		Do(ctx)
	require.NoError(t, err)
	require.True(t, resp.Success(), "failed with status %d: %s", resp.StatusCode, resp.Status)
	require.Equal(t, "image/png", resp.Header.Get(request.HeaderContentType))

	defer resp.Body.Close()
	img, _, err := image.Decode(resp.Body)
	require.NoError(t, err)
	got, err := qrcode.Decode(img)
	require.NoError(t, err)

	require.Equal(t, strings.ReplaceAll(content, "\n", "\r\n"), got)
}
