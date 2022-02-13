package main

import (
	"bufio"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/juunini/simple-go-line-notify/notify"
	"github.com/robfig/cron"
	log "github.com/sirupsen/logrus"
)

var (
	accessToken *string
	items       *string
)

type Document struct {
	*goquery.Document
}

func (doc *Document) GetItemsAvailable() (available int, err error) {
	doc.Find(`#app-store-front > div > div.flex.flex-col > div:nth-child(4) > div.mt-6 > div > div > div.mt-2.mr-4.flex.flex-wrap > div:nth-child(2)`).Each(func(i int, s *goquery.Selection) {
		text1 := s.Text()

		text2 := strings.ReplaceAll(text1, "item available", "")
		text3 := strings.ReplaceAll(text2, "items available", "")
		text4 := strings.ReplaceAll(text3, "/", "")
		text5 := strings.TrimSpace(text4)

		available, err = strconv.Atoi(text5)

	})
	return
}

func (doc *Document) GetItemsName() (name string, err error) {
	doc.Find(`#app-store-front > div > div.flex.flex-col > div.flex.flex-row.justify-between.pt-3.pl-15xem.pr-10xem > div.flex.flex-col.w-cal-32xem.pr-10xem > div.w-full.text-15xem.leading-18xem`).Each(func(i int, s *goquery.Selection) {
		text := s.Text()
		name = strings.TrimSpace(text)
	})
	return
}

func (doc *Document) GetItemsPrice() (price string, err error) {
	doc.Find(`#app-store-front > div > div.flex.flex-col > div.flex.flex-row.justify-between.pt-3.pl-15xem.pr-10xem > div.flex.flex-col.w-cal-32xem.pr-10xem > div.flex.flex-wrap.items-center > span`).Each(func(i int, s *goquery.Selection) {
		text := s.Text()
		price = strings.TrimSpace(text)
	})
	return
}

func GetDocument(link string) (*Document, error) {
	// Request the HTML page.
	res, err := http.Get(link)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return nil, fmt.Errorf("status code error: %d %s", res.StatusCode, res.Status)
	}

	// Load the HTML document
	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		return nil, err
	}

	return &Document{doc}, nil
}

func CheckGoldNotify(link string) error {
	spl := strings.Split(link, "/")
	l, err := os.OpenFile(spl[len(spl)-1]+".stock", os.O_APPEND|os.O_CREATE|os.O_RDWR, 0666)
	if err != nil {
		return err
	}

	defer l.Close()

	scanner := bufio.NewScanner(l)
	var sf string
	for scanner.Scan() {
		sf = scanner.Text()
	}
	if err := scanner.Err(); err != nil {
		return err
	}

	doc, err := GetDocument(link)
	if err != nil {
		return err
	}

	name, err := doc.GetItemsName()
	if err != nil {
		return err
	}

	stock, err := doc.GetItemsAvailable()
	if err != nil {
		return err
	}

	log.Println(name, stock)

	sfn, err := strconv.Atoi(sf)
	if err != nil {
		log.Warnln(err)
	}

	if sfn != stock {
		l.Truncate(0)
		fmt.Fprintf(l, "%d", stock)

		sfnA := sfn > 0
		stockA := stock > 0
		if sfnA != stockA {

			sym := "✅"
			if stock == 0 {
				sym = "❌"
			}

			if err := notify.SendText(*accessToken, fmt.Sprintf("\n%s\n%s%d\n%s", name, sym, stock, link)); err != nil {
				return err
			}
		}
	}

	return err
}

func main() {
	accessToken = flag.String("line-token", "", "Line notify access token")
	items = flag.String("items", "", "List of token to monitors EX: `https://shop.line.me/@aurorathailand/product/320403035,https://shop.line.me/@aurorathailand/product/320402984`")
	flag.Parse()

	// open a file
	l, err := os.OpenFile("gold.log", os.O_APPEND|os.O_CREATE|os.O_RDWR, 0666)
	if err != nil {
		fmt.Printf("error opening file: %v", err)
	}

	defer l.Close()

	log.SetFormatter(&log.JSONFormatter{})
	log.SetOutput(l)
	log.SetLevel(log.DebugLevel)

	log.Info("Create new cron")
	c := cron.New()

	c.AddFunc("*/5 * * * * *", func() {
		log.Infoln("RUNNING")
		itemsList := strings.Split(*items, ",")
		for _, url := range itemsList {

			err = CheckGoldNotify(url)
			if err != nil {
				log.Errorln(err)
			}
		}
	})

	log.Info("Start cron")

	go c.Start()
	sig := make(chan os.Signal)
	signal.Notify(sig, os.Interrupt, os.Kill)
	<-sig

	log.Infof("Cron Info: %+v\n", c.Entries())
}
