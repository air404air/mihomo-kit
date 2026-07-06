package main

import (
 "bytes"
 "encoding/base64"
 "fmt"
 "io"
 "net/http"
 "net/url"
 "os"
 "os/exec"
 "regexp"
 "strings"
 "time"
)

const (
 cfgPath = "/opt/etc/mihomo/config.yaml"
 subURLPath = "/opt/etc/mihomo/subscription.url"
 backupDir = "/opt/etc/mihomo/backups"
 uiPath = "/opt/etc/mihomo/ui"
)

func main() {
 http.HandleFunc("/", index)
 http.HandleFunc("/apply", apply)
 http.HandleFunc("/refresh", refresh)
 http.HandleFunc("/status", status)
 fmt.Println("Mihomo SubManager v1.2 listening on :9091")
 http.ListenAndServe(":9091", nil)
}

func index(w http.ResponseWriter, r *http.Request) {
 cur, _ := os.ReadFile(subURLPath)
 html := `<!doctype html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Mihomo SubManager</title><style>
body{font-family:Arial;background:#0b1020;color:#e8edf7;padding:24px}h1{font-size:42px}.card{background:#121a2a;border:1px solid #28364f;border-radius:22px;padding:22px;margin:18px 0}textarea{width:100%;height:90px;font-size:17px;padding:16px;border-radius:14px;border:1px solid #3b4b68;background:#050914;color:white;box-sizing:border-box}button{font-size:18px;font-weight:700;padding:16px 22px;border:0;border-radius:14px;background:#22c55e;color:#06130a;margin:8px 8px 8px 0}.small{color:#99a3b5}.danger{background:#334155;color:#fff}pre{white-space:pre-wrap;background:#050914;padding:18px;border-radius:14px;overflow:auto}</style></head><body>
<h1>Mihomo SubManager</h1><div class="card"><h2>Подписка</h2><form action="/apply" method="post"><textarea name="url" placeholder="Вставь ссылку подписки">`+escape(string(cur))+`</textarea><br><button type="submit">Скачать / применить</button></form><form action="/refresh" method="post"><button type="submit">Обновить текущую</button></form><p class="small">Zashboard: <a style="color:#7dd3fc" href="http://`+r.Host+`/ui/">9090/ui</a></p></div><div class="card"><h2>Статус</h2><pre>`+escape(getStatus())+`</pre></div></body></html>`
 w.Header().Set("Content-Type","text/html; charset=utf-8")
 io.WriteString(w, html)
}

func apply(w http.ResponseWriter, r *http.Request) {
 r.ParseForm()
 u := strings.TrimSpace(r.FormValue("url"))
 result := applyURL(u)
 w.Header().Set("Content-Type","text/plain; charset=utf-8")
 io.WriteString(w, result)
}

func refresh(w http.ResponseWriter, r *http.Request) {
 b, _ := os.ReadFile(subURLPath)
 result := applyURL(strings.TrimSpace(string(b)))
 w.Header().Set("Content-Type","text/plain; charset=utf-8")
 io.WriteString(w, result)
}

func status(w http.ResponseWriter, r *http.Request){
 w.Header().Set("Content-Type","text/plain; charset=utf-8")
 io.WriteString(w,getStatus())
}

func getStatus() string{
 out := &bytes.Buffer{}
 cmd:=exec.Command("/opt/bin/mihomo","-v")
 b,_:=cmd.CombinedOutput(); fmt.Fprintf(out,"submanager: v1.2\nmihomo: %s\n", strings.TrimSpace(string(b)))
 if b,err:=os.ReadFile(subURLPath); err==nil {fmt.Fprintf(out,"subscription: %s\n", strings.TrimSpace(string(b)))}
 if resp,err:=http.Get("http://127.0.0.1:9090/proxies"); err==nil {defer resp.Body.Close(); fmt.Fprintf(out,"api: %s\n", resp.Status)} else {fmt.Fprintf(out,"api: %v\n", err)}
 return out.String()
}

func applyURL(u string) string{
 var log bytes.Buffer
 fmt.Fprintf(&log,"URL=%s\n\n",u)
 if u=="" {return log.String()+"ERROR: empty url\n"}
 os.MkdirAll(backupDir,0755)
 os.WriteFile(subURLPath, []byte(u),0644)
 fmt.Fprintf(&log,"Downloading subscription with wget...\n")
 rawPath := "/opt/tmp/mihomo_sub.raw"
 wb, werr := exec.Command("wget", "-qO", rawPath, u).CombinedOutput()
 if len(wb)>0 { log.Write(wb) }
 if werr!=nil {return log.String()+"ERROR download: "+werr.Error()+"\n"}
 raw,err:=os.ReadFile(rawPath)
 if err!=nil {return log.String()+"ERROR read: "+err.Error()+"\n"}
 fmt.Fprintf(&log,"Downloaded bytes=%d\n",len(raw))
 cfg:=[]byte{}
 s:=strings.TrimSpace(string(raw))
 if strings.Contains(s,"proxies:") || strings.Contains(s,"proxy-providers:") || strings.Contains(s,"mixed-port:") {
  fmt.Fprintf(&log,"Detected: Clash/Mihomo YAML\n")
  cfg=raw
 } else {
  dec:=decodeSub(raw)
  lines:=extractLinks(dec)
  fmt.Fprintf(&log,"Detected links: %d\n",len(lines))
  if len(lines)==0 {return log.String()+"ERROR: no vless/trojan links found\n"}
  cfg=[]byte(buildConfig(lines))
 }
 tmp := "/opt/tmp/mihomo_config.new.yaml"
 os.WriteFile(tmp,cfg,0644)
 fmt.Fprintf(&log,"Testing config...\n")
 test:=exec.Command("/opt/bin/mihomo","-t","-d","/opt/etc/mihomo","-f",tmp)
 tb,err:=test.CombinedOutput(); log.Write(tb)
 if err!=nil {return log.String()+"\nERROR: mihomo config test failed\n"}
 back:=fmt.Sprintf("%s/config.%s.yaml",backupDir,time.Now().Format("20060102-150405"))
 if old,err:=os.ReadFile(cfgPath); err==nil {os.WriteFile(back,old,0644)}
 os.WriteFile(cfgPath,cfg,0644)
 fmt.Fprintf(&log,"Restarting mihomo...\n")
 rb,_:=exec.Command("/opt/etc/init.d/S99mihomo","restart").CombinedOutput(); log.Write(rb)
 exec.Command("/opt/etc/init.d/S98mihomo_redirect","restart").Run()
 fmt.Fprintf(&log,"\nOK: applied\nBackup: %s\n",back)
 return log.String()
}

func decodeSub(raw []byte) string{
 s:=strings.TrimSpace(string(raw))
 if strings.Contains(s,"vless://") || strings.Contains(s,"trojan://") {return s}
 compact:=regexp.MustCompile(`\\s+`).ReplaceAllString(s,"")
 if b,err:=base64.StdEncoding.DecodeString(compact); err==nil {return string(b)}
 if b,err:=base64.RawStdEncoding.DecodeString(compact); err==nil {return string(b)}
 return s
}
func extractLinks(s string) []string{
 var res []string
 for _,ln:=range strings.Split(s,"\n"){
  ln=strings.TrimSpace(ln)
  if strings.HasPrefix(ln,"vless://") || strings.HasPrefix(ln,"trojan://") {res=append(res,ln)}
 }
 return res
}
func yamlq(s string) string { return `"`+strings.ReplaceAll(strings.ReplaceAll(s,"\\","\\\\"),"\"","\\\"")+`"` }
func buildConfig(lines []string) string{
 var b strings.Builder
 b.WriteString(`mixed-port: 7890
socks-port: 7891
redir-port: 7892
allow-lan: true
bind-address: '*'
mode: rule
log-level: info
ipv6: false
external-controller: 0.0.0.0:9090
secret: ''
external-ui: /opt/etc/mihomo/ui
geodata-mode: true
geo-auto-update: true
geo-update-interval: 24

dns:
  enable: true
  listen: 0.0.0.0:7874
  enhanced-mode: fake-ip
  fake-ip-range: 198.18.0.1/16
  nameserver:
    - 1.1.1.1
    - 8.8.8.8

proxies:
`)
 names:=[]string{}
 for i,line:=range lines{
  name:=fmt.Sprintf("node%03d",i+1)
  if p:=strings.LastIndex(line,"#"); p>=0 { if n,err:=url.QueryUnescape(line[p+1:]); err==nil && strings.TrimSpace(n)!="" {name=strings.TrimSpace(n)} }
  names=append(names,name)
  u,err:=url.Parse(line); if err!=nil {continue}
  q:=u.Query(); proto:=u.Scheme; host:=u.Hostname(); port:=u.Port(); auth:=u.User.Username(); pass,_:=u.User.Password()
  b.WriteString("  - name: "+yamlq(name)+"\n")
  b.WriteString("    type: "+proto+"\n")
  b.WriteString("    server: "+yamlq(host)+"\n")
  b.WriteString("    port: "+port+"\n")
  if proto=="vless" { b.WriteString("    uuid: "+auth+"\n"); if q.Get("flow")!="" {b.WriteString("    flow: "+q.Get("flow")+"\n")} } else { if pass!="" {auth=auth+":"+pass}; b.WriteString("    password: "+yamlq(auth)+"\n") }
  network:=q.Get("type"); if network=="" {network="tcp"}
  b.WriteString("    network: "+network+"\n    udp: true\n")
  sec:=q.Get("security")
  if sec=="tls" || sec=="reality" { b.WriteString("    tls: true\n"); if q.Get("sni")!="" {b.WriteString("    servername: "+yamlq(q.Get("sni"))+"\n")}; if q.Get("fp")!="" {b.WriteString("    client-fingerprint: "+q.Get("fp")+"\n")}; b.WriteString("    skip-cert-verify: false\n") }
  if sec=="reality" { b.WriteString("    reality-opts:\n"); if q.Get("pbk")!="" {b.WriteString("      public-key: "+q.Get("pbk")+"\n")}; if q.Get("sid")!="" {b.WriteString("      short-id: "+q.Get("sid")+"\n")} }
  if network=="ws" {path:=q.Get("path"); if path=="" {path="/"}; b.WriteString("    ws-opts:\n      path: "+yamlq(path)+"\n")}
 }
 b.WriteString("\nproxy-groups:\n  - name: PROXY\n    type: select\n    proxies:\n      - AUTO\n      - DIRECT\n")
 for _,n:=range names {b.WriteString("      - "+yamlq(n)+"\n")}
 b.WriteString("  - name: AUTO\n    type: url-test\n    url: http://www.gstatic.com/generate_204\n    interval: 300\n    tolerance: 80\n    proxies:\n")
 for _,n:=range names {b.WriteString("      - "+yamlq(n)+"\n")}
 b.WriteString(`
rules:
  # Local / Private
  - GEOSITE,private,DIRECT
  - GEOIP,private,DIRECT

  # Popular blocked services
  - GEOSITE,youtube,PROXY
  - GEOSITE,telegram,PROXY
  - GEOSITE,discord,PROXY
  - GEOSITE,openai,PROXY
  - GEOSITE,facebook,PROXY
  - GEOSITE,tiktok,PROXY
  - GEOSITE,twitch,PROXY
  - GEOSITE,twitter,PROXY
  - GEOSITE,netflix,PROXY
  - GEOSITE,spotify,PROXY
  - GEOSITE,github,PROXY
  - GEOSITE,steam,PROXY
  - GEOSITE,microsoft,PROXY
  - GEOSITE,apple,PROXY
  - GEOSITE,google,PROXY

  # IP check services
  - DOMAIN-SUFFIX,2ip.ru,PROXY
  - DOMAIN-SUFFIX,2ip.io,PROXY
  - DOMAIN-SUFFIX,ipinfo.io,PROXY
  - DOMAIN-SUFFIX,ifconfig.me,PROXY

  # Russia direct
  - GEOIP,RU,DIRECT

  # Default
  - MATCH,PROXY
`)
 return b.String()
}
func escape(s string) string { r:=strings.NewReplacer("&","&amp;","<","&lt;",">","&gt;","\"","&quot;"); return r.Replace(s)}
