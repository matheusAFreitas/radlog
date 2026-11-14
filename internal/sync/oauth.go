package sync

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ---------- login com o Google (app de computador) ----------
//
// O fluxo é o de "app instalado": o navegador abre a página do Google e volta
// para um endereço local (127.0.0.1, porta sorteada) com o código, que vira
// um token com PKCE. Os escopos são só o e-mail (para mostrar na tela) e
// drive.file, que só enxerga os arquivos que o próprio app criou.

// Escopos pedidos ao Google.
const Scopes = "openid email https://www.googleapis.com/auth/drive.file"

// ErrRevoked é o "entre de novo" do Google (acesso revogado ou vencido).
var ErrRevoked = errors.New("o Google pediu para entrar de novo")

// ErrCanceled é o login cancelado (aqui ou na página do Google).
var ErrCanceled = errors.New("login cancelado")

// OAuth tem o que o login precisa. As URLs podem ser trocadas nos testes.
type OAuth struct {
	ClientID, Secret  string
	AuthURL, TokenURL string
	RevokeURL         string
	HTTP              *http.Client
}

// Google devolve a configuração com os endereços do Google.
func Google(id, secret string) *OAuth {
	return &OAuth{
		ClientID: id, Secret: secret,
		AuthURL:   "https://accounts.google.com/o/oauth2/v2/auth",
		TokenURL:  "https://oauth2.googleapis.com/token",
		RevokeURL: "https://oauth2.googleapis.com/revoke",
		HTTP:      &http.Client{Timeout: time.Minute},
	}
}

// Token é a resposta do Google. Email vem do id_token e só serve para a tela.
type Token struct {
	Access  string
	Refresh string
	Expiry  time.Time
	Email   string
}

// Login é um login em andamento.
type Login struct {
	URL      string // página do Google para abrir no navegador
	verifier string
	state    string
	redirect string
	ln       net.Listener
	srv      *http.Server
	got      chan loginResult
}

type loginResult struct {
	code string
	err  error
}

func randURL(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

// Start abre o endereço local e monta a URL do Google.
func (o *OAuth) Start() (*Login, error) {
	if o.ClientID == "" {
		return nil, errors.New("esta cópia do app não tem a chave do Google")
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	l := &Login{verifier: randURL(48), state: randURL(16), ln: ln, got: make(chan loginResult, 1)}
	l.redirect = fmt.Sprintf("http://127.0.0.1:%d/", ln.Addr().(*net.TCPAddr).Port)
	sum := sha256.Sum256([]byte(l.verifier))
	q := url.Values{
		"client_id":             {o.ClientID},
		"redirect_uri":          {l.redirect},
		"response_type":         {"code"},
		"scope":                 {Scopes},
		"code_challenge":        {base64.RawURLEncoding.EncodeToString(sum[:])},
		"code_challenge_method": {"S256"},
		"state":                 {l.state},
		"access_type":           {"offline"},
		"prompt":                {"consent"},
	}
	l.URL = o.AuthURL + "?" + q.Encode()
	l.srv = &http.Server{Handler: http.HandlerFunc(l.serve), ReadHeaderTimeout: 10 * time.Second}
	go func() { _ = l.srv.Serve(ln) }()
	return l, nil
}

const loginPage = `<!doctype html><meta charset="utf-8"><title>RadLog</title>
<body style="background:#0E1316;color:#E6EDF2;font:16px system-ui,sans-serif;display:grid;place-items:center;height:90vh;text-align:center">
<div><h2>%s</h2><p style="color:#8B98A5;margin-top:-8px">%s</p><p>Pode fechar esta aba e voltar para o RadLog.</p><p style="color:#8B98A5">You can close this tab and go back to RadLog.</p></div>`

func (l *Login) serve(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	if r.URL.Path != "/" || (q.Get("code") == "" && q.Get("error") == "") {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if q.Get("state") != l.state {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, loginPage, "Login não reconhecido", "Login not recognized")
		return
	}
	res := loginResult{code: q.Get("code")}
	title, en := "Pronto, login feito", "Done, you are signed in"
	if e := q.Get("error"); e != "" {
		res = loginResult{err: ErrCanceled}
		if e != "access_denied" {
			res.err = fmt.Errorf("o Google recusou o login (%s)", e)
		}
		title, en = "Login cancelado", "Login canceled"
	}
	fmt.Fprintf(w, loginPage, title, en)
	select {
	case l.got <- res:
	default:
	}
}

// Wait espera o navegador voltar (ou ctx acabar) e fecha o endereço local.
func (l *Login) Wait(ctx context.Context) (string, error) {
	defer l.Close()
	select {
	case r := <-l.got:
		return r.code, r.err
	case <-ctx.Done():
		return "", ErrCanceled
	}
}

// Close fecha o endereço local.
func (l *Login) Close() {
	if l.srv != nil {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		_ = l.srv.Shutdown(ctx)
		cancel()
	}
}

// Exchange troca o código do navegador pelos tokens.
func (o *OAuth) Exchange(ctx context.Context, l *Login, code string) (Token, error) {
	return o.token(ctx, url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"code_verifier": {l.verifier},
		"redirect_uri":  {l.redirect},
	})
}

// Refresh pede um token de acesso novo. ErrRevoked quando o Google pede
// para entrar de novo.
func (o *OAuth) Refresh(ctx context.Context, refresh string) (Token, error) {
	t, err := o.token(ctx, url.Values{"grant_type": {"refresh_token"}, "refresh_token": {refresh}})
	if err == nil && t.Refresh == "" {
		t.Refresh = refresh
	}
	return t, err
}

func (o *OAuth) token(ctx context.Context, v url.Values) (Token, error) {
	v.Set("client_id", o.ClientID)
	if o.Secret != "" {
		v.Set("client_secret", o.Secret)
	}
	req, err := http.NewRequestWithContext(ctx, "POST", o.TokenURL, strings.NewReader(v.Encode()))
	if err != nil {
		return Token{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := o.HTTP.Do(req)
	if err != nil {
		return Token{}, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	var r struct {
		Access  string `json:"access_token"`
		Refresh string `json:"refresh_token"`
		Expires int64  `json:"expires_in"`
		IDToken string `json:"id_token"`
		Error   string `json:"error"`
	}
	_ = json.Unmarshal(body, &r)
	if resp.StatusCode != 200 || r.Access == "" {
		if r.Error == "invalid_grant" || r.Error == "unauthorized_client" {
			return Token{}, ErrRevoked
		}
		return Token{}, &HTTPError{Code: resp.StatusCode, Msg: r.Error}
	}
	if r.Expires <= 0 {
		r.Expires = 3600
	}
	return Token{Access: r.Access, Refresh: r.Refresh, Expiry: time.Now().Add(time.Duration(r.Expires-60) * time.Second), Email: idEmail(r.IDToken)}, nil
}

// Revoke tira o acesso do app na conta Google (some da lista de apps da
// conta). Vale para todos os aparelhos com o mesmo cliente nessa conta.
func (o *OAuth) Revoke(ctx context.Context, token string) error {
	if o.RevokeURL == "" || token == "" {
		return nil
	}
	req, err := http.NewRequestWithContext(ctx, "POST", o.RevokeURL, strings.NewReader(url.Values{"token": {token}}.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := o.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	// 400 = o token já não valia
	if resp.StatusCode != 200 && resp.StatusCode != 400 {
		return &HTTPError{Code: resp.StatusCode}
	}
	return nil
}

// idEmail lê o e-mail do id_token (veio direto do Google por HTTPS, então
// não precisa conferir a assinatura só para mostrar na tela).
func idEmail(jwt string) string {
	p := strings.Split(jwt, ".")
	if len(p) < 2 {
		return ""
	}
	b, err := base64.RawURLEncoding.DecodeString(p[1])
	if err != nil {
		return ""
	}
	var c struct {
		Email string `json:"email"`
	}
	_ = json.Unmarshal(b, &c)
	return c.Email
}

// HTTPError é uma resposta de erro do Google.
type HTTPError struct {
	Code int
	Msg  string
}

func (e *HTTPError) Error() string {
	if e.Msg != "" {
		return fmt.Sprintf("Google respondeu %d (%s)", e.Code, e.Msg)
	}
	return fmt.Sprintf("Google respondeu %d", e.Code)
}
