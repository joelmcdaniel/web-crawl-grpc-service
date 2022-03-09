package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	pb "web-crawl-grpc-service/webcrawlerpb"

	"golang.org/x/net/html"
	"google.golang.org/grpc"
)

var (
	port = flag.Int("port", 50051, "The server port")
)

type server struct {
	webcrawler *webcrawler
}

type webcrawler struct {
	currentRoot string
	visited     map[string]string
	stop        bool
}

func newWebCrawler(root string) *webcrawler {
	return &webcrawler{
		currentRoot: root,
		visited:     make(map[string]string),
		stop:        false,
	}
}

func (s *server) Start(_ context.Context, req *pb.TreeRequest) (*pb.StartResponse, error) {
	//log.Printf("start request; start url is: %s", req.StartUrl)
	s.webcrawler = newWebCrawler(req.StartUrl)

	go s.crawl(req.StartUrl, req.StartUrl, &s.webcrawler.visited)

	return &pb.StartResponse{
		Message: fmt.Sprintf("Web crawler started...\n root = %s", req.StartUrl),
	}, nil
}

func (s *server) Stop(_ context.Context, req *pb.StopRequest) (*pb.StopResponse, error) {

	s.webcrawler.stop = true
	// log.Printf("stop request; stop url is: %s", req.StopUrl)
	return &pb.StopResponse{
		Message: fmt.Sprintf("Web crawler stopped...\n url = %s", req.StopUrl),
	}, nil
}

func (s *server) List(_ *pb.ListRequest, stream pb.WebCrawler_ListServer) error {

	if s.webcrawler != nil {
		//log.Printf("list request; list url is: %s", s.webcrawler.currentRoot)
		for k, v := range s.webcrawler.visited {

			ln := &pb.TreeRequest{
				TreeLink:  k,
				PageTitle: v,
			}
			if err := stream.Send(ln); err != nil {
				return err
			}
		}
	}
	return nil
}

func main() {
	flag.Parse()

	lis, err := net.Listen("tcp", fmt.Sprintf("localhost:%d", *port))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	svr := grpc.NewServer()

	pb.RegisterWebCrawlerServer(svr, &server{})
	log.Printf("server listening at %v", lis.Addr())
	if err := svr.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}

// crawl - given a url and a basurl, recursively scans the page
// following all the links and fills the `visited` map
func (s *server) crawl(url, baseurl string, visited *map[string]string) {

	if s.webcrawler.stop {
		return
	}

	page, err := parse(url)
	if err != nil {
		fmt.Printf("error parsing page %s %s\n", url, err)
		return
	}

	title := pageTitle(page)
	(*visited)[url] = title

	//recursively find links
	links := pageLinks(nil, page)

	for _, link := range links {
		if s.webcrawler.stop {
			break
		} else {
			if (*visited)[link] == "" && strings.HasPrefix(link, baseurl) {
				s.crawl(link, baseurl, visited)
			}
		}
	}
}

// parse - given a string pointing to a URL will fetch and parse it
// returning an html.Node pointer
func parse(url string) (*html.Node, error) {
	r, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("cannot get page")
	}
	b, err := html.Parse(r.Body)
	if err != nil {
		return nil, fmt.Errorf("cannot parse page")
	}
	return b, err
}

// pageTitle - given a reference to a html.Node, scans it until it
// finds the title tag, and returns its value
func pageTitle(n *html.Node) string {
	var title string
	if n.Type == html.ElementNode && n.Data == "title" {
		return n.FirstChild.Data
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		title = pageTitle(c)
		if title != "" {
			break
		}
	}
	return title
}

// pageLinks - recursively scans an `html.Node` and will return
// a list of links found, with no duplicates
func pageLinks(links []string, n *html.Node) []string {
	if n.Type == html.ElementNode && n.Data == "a" {
		for _, a := range n.Attr {
			if a.Key == "href" {
				if !sliceContains(links, a.Val) {
					links = append(links, a.Val)
				}
			}
		}
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		links = pageLinks(links, c)
	}
	return links
}

// sliceContains - returns true if `slice` contains `value`
func sliceContains(slice []string, value string) bool {
	for _, v := range slice {
		if v == value {
			return true
		}
	}
	return false
}
