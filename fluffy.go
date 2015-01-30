/*
 * Awesom IRC bot.
 *
 * Tested by #foo.
 *
 * This code is as available and free as a grain of sand in the subcontinental
 * Sahara.
 *
 * (Hope this will cover every jurisdictions better than public domain)
 *
 */
package main

import (
	"crypto/tls"
	"flag"
	irc "github.com/fluffle/goirc/client"
	"html"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	nick = flag.String("nick", "fluffy", "IRC nick name")
	room = flag.String("chan", "#bar", "IRC channel")
	serv = flag.String("serv", "irc.awesom.eu", "IRC server")
	nlog = flag.Int("nlog", 500, "Maximum internal log size")

	//	book = flag.String("book", "http://bookmarks.awesom.eu", "Bookmarks server")
	fort  = flag.String("fort", "http://fortune.awesom.eu", "Fortunes server")
	paste = flag.String("paste", "http://paste.awesom.eu", "Paste server")

	// Let's not print a full HTML page to the channel.
	// bullet proof
	maxout = flag.Int("maxout", 15, "Maximum number of IRC lines sent in a row, <= 0 for no restriction")

	// tabs replaced by 4 spaces
	Tab = "    "
)

type Dumper interface {
	Save() bool
	Load() bool
}

// -- Logging (!lastlog)

// Buffer for lastlog
type Logger struct {
	sync.Mutex
	data []string
	pos  int
	full bool
}

var logger *Logger

func NewLogger(n int) *Logger {
	return &Logger{data: make([]string, n), pos: 0}
}

// pretty nickname output, isn't it?
func q(s string) string {
	return "<" + s + ">"
}

func (f *Logger) AddLine(t time.Time, n string, l string) {
	f.Lock()
	if f.pos >= len(f.data) {
		f.pos, f.full = 0, true
	}
	// time representation is standardized. Who'd guess?
	f.data[f.pos] = t.Format(time.RFC822) + " " + n + " " + l
	f.pos++
	f.Unlock()
}

// Enjoy your debugging:
// reverse looping, -1 indexes >= 0 AND > 0, reverse string concatenation
func (f *Logger) Dump(n int) (res string) {
	f.Lock()
	if !f.full {
		// this was used to avoid a bug.
		// was not the root cause however.
		// seems reasonable to keep it, but may
		// be that useless.
		if n > f.pos {
			n = f.pos
		}
		// end.
		for i := f.pos - 1; i >= 0 && n > 0; i, n = i-1, n-1 {
			res = f.data[i] + "\n" + res
		}
	} else {
		// this was used to avoid a bug.
		// was not the root cause however.
		// seems reasonable to keep it, but may
		// be that useless.
		if n > len(f.data) {
			n = len(f.data)
		}
		// end.
		for i := f.pos - 1; i >= 0 && n > 0; i, n = i-1, n-1 {
			res = f.data[i] + "\n" + res
		}
		for i := len(f.data) - 1; i >= f.pos && n > 0; i, n = i-1, n-1 {
			res = f.data[i] + "\n" + res
		}
	}
	f.Unlock()
	return
}

// -- Telling (!tell)

type Teller struct {
	sync.Mutex
	msgs map[string][]string
}

var teller *Teller

func NewTeller() *Teller {
	return &Teller{msgs: make(map[string][]string)}
}

func (t *Teller) Tell(n, m string) {
	t.Lock()
	t.msgs[n] = append(t.msgs[n], m)
	t.Unlock()
}

func (t *Teller) Pop(n string) (msgs []string) {
	exists := false
	t.Lock()
	if msgs, exists = t.msgs[n]; exists {
		delete(t.msgs, n)
		// Once, there was a return here
		// but no goto, causing the Lock
		// never to be Unlocked,
		// and the fluffy looping blindly.
		//return
	}
	t.Unlock()
	return
}

// -- Persistence
// Funny naming. After the great commenting invasion.
func SaveTheFluffy() {
}

func LoadTheFluffy() {
}

// Multiline private  message; substitue tab for spaces
func Privmsg2(c *irc.Conn, t, m string) {
	// bullet proof
	// i think this one was first find by an Izu
	// an Asgeir for sure was near.
	n := *maxout
	for _, l := range strings.Split(m, "\n") {
		line := strings.Replace(l, "\t", Tab, -1)
		c.Privmsg(t, line)
		logger.AddLine(time.Now(), q(*nick), line)

		// bullet proof
		// what a breaking integer!
		n = n - 1
		if n == 0 {
			break
		}
	}
}

// Q: Why &raw=1 and not just &raw?
// A: Because the author of fortune didn't (still don't) know
//    how to differentiate an r.FormValue("raw") with no value
//    or a missing r.FormValue("raw").
// PS: the raw=1 was originally added for the fluffy to fortune(1)
func addFortune(f string) string {
	resp, err := http.PostForm(*fort+"/add?raw=1",
		url.Values{"fortune": {f}})
	if err != nil {
		log.Println(err)
		return "no."
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "no."
	}

	return "New fortune at " +
		*fort + resp.Request.URL.Path + " :\n" + string(body)
}

// XXX try fortune(1) if not successful?
// Requesting fortune(1) to be installed on bli.awesom.eu
// caused the server to be down: Within the jail, one can't
// install the games.txz because of read-only filesystems.
// Asking the owner lead him to fully upgrade the whole OSes
// ultimately leading to a broken ZFS and to an un-bootable system.
//
// Now we have deployed newsome, so...
//
// But still no fortune(1). I don't dare to ask again.
//
// Please, note the &raw=1, again.
func getFortune(query string) string {
	n, err := strconv.ParseInt(query, 10, 32)
	url := *fort + "/"
	if err == nil && n >= 0 {
		url += strconv.FormatInt(n, 10)
	}
	resp, err := http.Get(url + "?raw=1")
	if err != nil {
		log.Println(err)
		return "random fortune"
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "random fortune"
	}

	return string(body)
}

func fortune(c *irc.Conn, l *irc.Line, args []string) {
	var f string

	if len(args) > 1 {
		// o_oh, we're adding a fortune!
		if strings.HasPrefix(args[1], "+ ") {
			f = addFortune(strings.Trim(args[1], "+ "))
		} else {
			f = getFortune(args[1])
		}
	} else {
		f = getFortune("")
	}

	// for you only, or for everyone is this fortune
	if l.Args[0] == *nick {
		Privmsg2(c, l.Nick, f)
	} else {
		Privmsg2(c, l.Args[0], f)
	}
}

// XXX allows !help [cmd]
func help(c *irc.Conn, l *irc.Line, args []string) {
	for _, v := range commands {
		Privmsg2(c, l.Nick, v.help)
	}
}

func addPaste(s, room string) string {
	// talking to paste was not easy
	// it required a talk with its owner
	// to use the paste to its whole extent
	resp, err := http.PostForm(*paste+"",
		url.Values{
			"paste":   {s},
			"script":  {""},
			"user":    {*nick},
			"comment": {room}})
	if err != nil {
		log.Println(err)
		return "no."
	}
	defer resp.Body.Close()

	// at first, there was no ReadAll
	// and all was not always good until acieroid had
	// a look at the source.
	// " A &script is missing, plus it's not
	//   redirecting but outputing
	// and a full ioutil.ReadAll was then setup
	// The following lines are for purely historical
	// purposes.
	if resp.Request.URL.Path != "" {
		// here is a debugging line
		// in case one ever found the cases
		// where this works. (i assure that it sometimes did)
		log.Println("resp.Request.URL.Path:", *paste+resp.Request.URL.Path)
		return *paste + resp.Request.URL.Path
	}
	// end of the historical artefact

	// This is the ReadAll all talk about.
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Println(err)
	}
	// once, the "/" was missing.
	// this lead to weird hostname: http://paste.awesom.eumjPp
	// note that thanks to the previous line, we know that it
	// was *before* the &user= was used!
	return *paste + "/" + string(body)
}

func lastlog(c *irc.Conn, l *irc.Line, args []string) {
	n := *nlog + 1/2
	if len(args) > 1 {
		if m, err := strconv.Atoi(args[1]); err == nil {
			n = m
		}
	}

	// bullet proof
	// found by acieroid
	// this one makes some sense: [-n,0] lines of log
	if n < 0 {
		n = -n
	}
	// but this one is plainly hacky:
	if n == 0 {
		n = 1
	}

	Privmsg2(c, l.Args[0], l.Nick+": "+addPaste(logger.Dump(n), l.Args[0]))
}

// tell you, tell me, tell us all about it.
func tell(c *irc.Conn, l *irc.Line, args []string) {
	args2 := strings.SplitN(args[1], " ", 2)
	if len(args2) > 1 {
		teller.Tell(args2[0], l.Nick+" told you: "+args2[1])
		Privmsg2(c, l.Args[0], l.Nick+": I will.")
	}
}

// mostly useless.
// at least, it could have been the first command to have been
// implemented, as it was simple and so.
// i think help and fortune were the first commands. followed
// by lastlog, tell.
// but well, it's fun.
func ping(c *irc.Conn, l *irc.Line, args []string) {
	Privmsg2(c, l.Args[0], l.Nick+": pong!")
}

// Superior Ping/Pong Mechanism.
// We should definitely register a patent on this.
func pong(c *irc.Conn, l *irc.Line, args []string) {
	Privmsg2(c, l.Args[0], l.Nick+": ping!")
}

var tags map[string][]string = map[string][]string{}

func look(c *irc.Conn, l *irc.Line, args []string) {
	urls := map[string]bool{}
	for _, t := range strings.Split(args[1], " ") {
		if matches, exists := tags[t]; exists {
			for _, url := range matches {
				urls[url] = true
			}
		}
	}
	for url, _ := range urls {
		Privmsg2(c, l.Args[0], l.Nick+": "+url)
	}
}

type Command struct {
	help string
	exec func(*irc.Conn, *irc.Line, []string)
}

var commands map[string]Command

// can't assign statically because cyclic
// init dependency with help()
func init() {
	commands = map[string]Command{
		"fortune": {
			"!fortune <+> <fortune> : Add a fortune\n" +
				"!fortune [n]           : Get a fortune",
			fortune,
		},
		"lastlog": {
			"!lastlog [n]           : Paste to " + *paste + " at most n:" + strconv.Itoa(*nlog) + " lines of log",
			lastlog,
		},
		"tell": {
			"!tell <nick> <stuff>   : Tell stuff to nick when he comes back",
			tell,
		},
		"help": {
			"!help                  : Display this message",
			help,
		},
		"look": {
			"!look <tags>		: Look for tagged urls (tag1|tag2|...|tagn)",
			look,
		},
		"ping": {
			"!ping                  : Send back a pong",
			ping,
		},
		"pong": {
			"!pong                  : Send back a ping",
			pong,
		},
	}
}

// websites for which we don' want to get the title
var donotwant = []string{
	"wikipedia.org",
	"paste.awesom.eu",
	"i.imgur.com",
	"://google.com",
}

func htmlTitle(url string) string {
	// allows invalidate https certificate.
	// thanks Izu
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}
	resp, err := client.Get(url)

	if err != nil {
		log.Println(err)
		return ""
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Println(err)
		return ""
	}

	b := strings.Index(string(body), "<title>")
	if b == -1 {
		b = strings.Index(string(body), "<TITLE>")
	}
	e := strings.Index(string(body), "</title>")
	if e == -1 {
		e = strings.Index(string(body), "</TITLE>")
	}
	if b == -1 || e == -1 {
		return ""
	}

	return strings.Replace(html.UnescapeString(string(body[b+7:e])),
		"\n", " ", -1)
}

// "getLine" is a misleading name.
func getLine(c *irc.Conn, l *irc.Line) {
	logger.AddLine(l.Time, q(l.Nick), l.Text())
	if len(l.Args) >= 2 {
		args := strings.SplitN(l.Args[1], " ", 2)
		//args := strings.Split(l.Args[1], " ")

		// looking for a !cmd
		// fun fact, found by Izu i suppose, is that
		// even when there's no '!', like 'fortune'
		// instead of '!fortune', things works.
		// this is because of this very TrimPrefix.
		cmd := strings.TrimPrefix(args[0], "!")
		if f, exists := commands[cmd]; exists {
			f.exec(c, l, args)
		}
	}
	// look for urls
	words := strings.Fields(l.Text())
	for _, w := range words {
		if strings.HasPrefix(w, "http://") || strings.HasPrefix(w, "https://") {
			want := true
			for _, t := range donotwant {
				if strings.Contains(w, t) {
					want = false
				}
			}
			if want {
				title := htmlTitle(w)
				if title != "" {
					Privmsg2(c, l.Args[0], title)
				}
				// tag it
				for _, t := range words {
					if strings.HasPrefix(t, "#") {
						tags[t] = append(tags[t], w)
					}
				}
			}
		}
	}
}

func main() {
	flag.Parse()

	// Starting the fluffy.
	logger = NewLogger(*nlog)
	teller = NewTeller()

	// IRConfigure it
	cfg := irc.NewConfig(*nick)
	cfg.Server = *serv
	c := irc.Client(cfg)

	c.HandleFunc("connected",
		func(conn *irc.Conn, line *irc.Line) {
			// "Jack, we're on the awesom IRC network!"
			log.Println("Connected. Joining", *room, "...")
			conn.Join(*room)
		})

	c.HandleFunc("invite",
		func(conn *irc.Conn, line *irc.Line) {
			log.Println("Joining", line.Args[1], "...")
			conn.Join(line.Args[1])
		})

	c.HandleFunc("join",
		func(conn *irc.Conn, line *irc.Line) {
			// we will try to OP you if we can.
			// always.
			c.Mode(line.Args[0], "+o "+line.Nick)
			// let's remember you came
			logger.AddLine(line.Time, line.Nick, "has joined")
			// maybe someone needed you but you weren't here
			if msgs := teller.Pop(line.Nick); len(msgs) > 0 {
				for _, msg := range msgs {
					Privmsg2(conn, line.Args[0], line.Nick+": "+msg)
				}
			}
		})

	c.HandleFunc("part",
		func(conn *irc.Conn, line *irc.Line) {
			// remember you /part
			logger.AddLine(line.Time, line.Nick, "has left")
		})

	c.HandleFunc("quit",
		func(conn *irc.Conn, line *irc.Line) {
			// remember you /quit
			logger.AddLine(line.Time, line.Nick, "has quit")
		})

	quit := make(chan bool)
	c.HandleFunc("disconnected",
		func(conn *irc.Conn, line *irc.Line) {
			log.Println("Leaving...")
			quit <- true
		})

	// getLine was too long to be here.
	// so we put it in an external function.
	// how interesting, eh?!
	c.HandleFunc("PRIVMSG", getLine)

	if err := c.Connect(); err != nil {
		log.Fatal("Can connect to %s%s: %s", serv, room, err)
	}

	<-quit
}
