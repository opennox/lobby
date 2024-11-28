package main

import (
	"context"
	"log"
	"net/http"
	_ "net/http/pprof"
	"time"

	"github.com/opennox/xwis"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/cobra"

	"github.com/opennox/lobby"
)

func init() {
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "run the lobby web server",
	}
	fHost := cmd.Flags().String("host", ":8080", "host the server will listen on")
	fMonitor := cmd.Flags().String("monitor", "127.0.0.1:6060", "host the monitoring api will listen on")
	fGlobal := cmd.Flags().Bool("global", false, "run the server in a global mode (monitor other servers)")
	fXWIS := cmd.Flags().Bool("xwis", true, "list games from XWIS as well")
	fXLogin := cmd.Flags().String("xlogin", "", "XWIS login to use")
	fXPass := cmd.Flags().String("xpass", "", "XWIS password to use")
	fXCache := cmd.Flags().Duration("xcache", lobby.DefaultTimeout/2, "XWIS cache duration")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		var lb lobby.Lobby = lobby.NewLobby()
		if *fXWIS {
			log.Println("logging in to XWIS")
			c, err := xwis.NewClient(context.Background(), *fXLogin, *fXPass)
			if err != nil {
				return err
			}
			defer c.Close()
			var lx lobby.Lister = lobby.NewXWISWithClient(c)
			if *fXCache > 0 {
				lx = lobby.Cache(lx, *fXCache)
			}
			lb = lobby.Overlay(lb, lx)
		}
		lsrv := lobby.NewServer(lb)
		// TODO: auto TLS with Let's Encrypt
		srv := &http.Server{
			Addr: *fHost,
			Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				log.Println(r.RemoteAddr, r.Method, r.URL)
				lsrv.ServeHTTP(w, r)
			}),
		}
		log.Println("serving lobby on", srv.Addr)
		if *fMonitor != "" {
			http.Handle("/metrics", promhttp.Handler())
			log.Println("serving monitoring on", *fMonitor)
			go func() {
				if err := http.ListenAndServe(*fMonitor, nil); err != nil {
					log.Println(err)
				}
			}()
			if *fGlobal {
				// For the global server, we want to monitor the list of all games.
				// Since XWIS list will only be retrieved when request comes in, we must periodically do list requests here.
				go func() {
					ctx := context.Background()
					ticker := time.NewTicker(lobby.DefaultTimeout / 2)
					defer ticker.Stop()
					for range ticker.C {
						_, _ = lb.ListGames(ctx)
					}
				}()
			}
		}
		return srv.ListenAndServe()
	}
	Root.AddCommand(cmd)
}
