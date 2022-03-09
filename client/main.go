package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	pb "web-crawl-grpc-service/webcrawlerpb"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var (
	addr, start, stop string
	list              bool
)

func init() {

	flag.StringVar(&addr, "addr", "localhost:50051", "the address to connect to")

	flag.StringVar(&start, "start", "", "Start crawling a url; value = `www.example.com`")

	flag.StringVar(&stop, "stop", "", "Stop crawling a url; value = `www.example.com`")

	flag.BoolVar(&list, "list", true, "list the `site tree` of crawled urls")

	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage of %s:\n", os.Args[0])
		flag.PrintDefaults()
		os.Exit(1)
	}
}

func parseURL(u string) (string, error) {
	url, err := url.Parse(u)
	if err != nil {
		return "", errors.New("url is not valid")
	}
	if url.Scheme == "" {
		url.Scheme = "https"
	}
	return url.String(), nil
}

func startWebCrawler(cl pb.WebCrawlerClient, req *pb.TreeRequest) {
	resp, err := cl.Start(context.Background(), req)
	if err != nil {
		log.Fatalf("Could not start web crawler %s", err)
	}
	log.Printf(resp.Message)
}

func stopWebCrawler(cl pb.WebCrawlerClient, req *pb.StopRequest) {
	resp, err := cl.Stop(context.Background(), req)
	if err != nil {
		log.Fatalf("Could not stop web crawler %s", err)
	}
	log.Printf(resp.Message)
}

func listSiteTree(cl pb.WebCrawlerClient, list *pb.ListRequest) {
	stream, err := cl.List(context.Background(), list)
	if err != nil {
		log.Fatalf("Could not List Site Tree %v", err)
	}

	pr, pw := io.Pipe()
	go func() {
		defer pw.Close()
		total := 0
		for {
			// Receiving the stream of data
			tree, err := stream.Recv()
			if err == io.EOF {
				pw.Write([]byte(fmt.Sprintf("Total unique links: %d\n", total)))
				break
			}
			if err != nil {
				log.Fatalf("Error receiving stream of site tree data %s", err)
			}
			total += 1
			pw.Write([]byte(fmt.Sprintf("%s:\n\t %s\n", tree.PageTitle, tree.TreeLink)))
		}
	}()

	if _, err := io.Copy(os.Stdout, pr); err != nil {
		log.Fatal(err)
	}
}

func main() {

	flag.Parse()

	// Set up a connection to the server.
	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	defer conn.Close()

	cl := pb.NewWebCrawlerClient(conn)

	switch os.Args[1] {
	case "-start", "--start":

		if len(start) == 0 {
			flag.Usage()
		} else {
			url, err := parseURL(start)
			if err != nil {
				log.Print(err)
				flag.Usage()
			}
			req := &pb.TreeRequest{StartUrl: url}
			startWebCrawler(cl, req)
		}

	case "-stop", "--stop":
		if len(stop) == 0 {
			flag.Usage()
		} else {
			url, err := parseURL(stop)
			if err != nil {
				log.Print(err)
				flag.Usage()
			}

			req := &pb.StopRequest{StopUrl: url}
			stopWebCrawler(cl, req)
		}

	case "-list", "--list":
		list := &pb.ListRequest{}
		listSiteTree(cl, list)
	}

}
