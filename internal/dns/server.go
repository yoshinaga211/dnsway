package dns

import (
"context"
"dnsway-pc/internal/engine"
"dnsway-pc/internal/models"
"fmt"
"github.com/miekg/dns"
)

type DNSServer struct {
	engine *engine.Engine
	server *dns.Server
	client *dns.Client // 新增上游查询客户端
}

func NewDNSServer(eng *engine.Engine) *DNSServer {
	return &DNSServer{
		engine: eng,
		client: &dns.Client{},
	}
}

func (s *DNSServer) Start(port string) error {
	dns.HandleFunc(".", s.handleDNSRequest)
	s.server = &dns.Server{
		Addr: ":" + port,
		Net:  "udp",
	}
	fmt.Printf("🚀 [DNS_SERVER] 正在 UDP %s 端口启动真实解析服务...\n", port)
	return s.server.ListenAndServe()
}

func (s *DNSServer) handleDNSRequest(w dns.ResponseWriter, r *dns.Msg) {
	msg := new(dns.Msg)
	msg.SetReply(r)
	
	for _, q := range r.Question {
		domain := q.Name[:len(q.Name)-1]
		decision := s.engine.Decide(context.Background(), "test", domain, "")
		
		if decision.Decision == models.Block {
			rr, _ := dns.NewRR(fmt.Sprintf("%s 60 IN A 0.0.0.0", q.Name))
			msg.Answer = append(msg.Answer, rr)
		} else if decision.Decision == models.REWRITE {
			rr, _ := dns.NewRR(fmt.Sprintf("%s 60 IN CNAME %s.", q.Name, decision.Domain))
			msg.Answer = append(msg.Answer, rr)
		} else {
			// 核心改进：请求放行时，向上游 8.8.8.8 转发真实查询
			in, _, err := s.client.Exchange(r, "8.8.8.8:53")
			if err == nil && in != nil {
				msg.Answer = append(msg.Answer, in.Answer...)
			} else {
				// 兜底：上游查询失败时返回模拟 IP
				rr, _ := dns.NewRR(fmt.Sprintf("%s 60 IN A 1.2.3.4", q.Name))
				msg.Answer = append(msg.Answer, rr)
			}
		}
	}
	w.WriteMsg(msg)
}
