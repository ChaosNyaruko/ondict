package main

import (
	"log"
	"net/http"
	"strings"
	"time"
)

type proxy struct {
	timeout *time.Timer
}

func (s *proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !s.timeout.Stop() {
		select {
		case t := <-s.timeout.C: // try to drain from the channel
			log.Printf("drained from timer: %v", t)
		default:
		}
	}
	if *idleTimeout > 0 {
		s.timeout.Reset(*idleTimeout)
	}
	log.Printf("query HTTP path: %v", r.URL.Path)
	if r.URL.Path == "/dict" {
		q := r.URL.Query()
		word := q.Get("query")
		e := q.Get("engine")
		f := q.Get("format")
		log.Printf("query dict: %v, engine: %v, format: %v", word, e, f)

		res := query(word, e, f)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(res))
		if f == "html" {
			w.Write([]byte("<style>" + oald9css + "</style>"))
		}
		// w.Write([]byte("<style>" + odecss + "</style>"))
		// w.Write([]byte(fmt.Sprintf(`<link ref="stylesheet" type="text/css", href=/d/static/oald9.css />`)))
		return
	}
	// if strings.HasSuffix(r.URL.Path, ".css") {
	// 	log.Printf("static info: %v", r.URL.Path)
	// 	http.FileServer(http.Dir("./static")).ServeHTTP(w, r)
	// 	return
	// }
	http.FileServer(http.Dir(".")).ServeHTTP(w, r)
}

const odecss = `
@font-face {
	font-family: "ODECondensed";
	src: url("fonts/opensanscondensed-light.ttf");
	font-weight: normal;
	font-style: normal;
}

@font-face {
	font-family: "ODESans";
	src: url("fonts/opensans-regular.ttf");
	font-weight: normal;
	font-style: normal;
}

@font-face {
	font-family: "ODESans";
	src: url("fonts/opensans-bold.ttf");
	font-weight: bold;
	font-style: normal;
}

@font-face {
	font-family: "ODESans";
	src: url("fonts/opensans-italic.ttf");
	font-weight: normal;
	font-style: italic;
}

@font-face {
	font-family: "ODESans";
	src: url("fonts/opensans-bolditalic.ttf");
	font-weight: bold;
	font-style: italic;
}

@font-face {
	font-family: "ODESerif";
	src: url("fonts/lora-regular.ttf");
	font-weight: normal;
	font-style: normal;
}

@font-face {
	font-family: "ODESerif";
	src: url("fonts/lora-bold.ttf");
	font-weight: bold;
	font-style: normal;
}

@font-face {
	font-family: "ODESerif";
	src: url("fonts/lora-italic.ttf");
	font-weight: normal;
	font-style: italic;
}

@font-face {
	font-family: "ODESerif";
	src: url("fonts/lora-bolditalic.ttf");
	font-weight: bold;
	font-style: italic;
}

@font-face {
	font-family: SansPhon;
	src: url('fonts/gentiumplus-r.ttf') format('truetype');
}

.Od3 {
	/* font-family: Open Sans, ODESans, sans-serif; */
	font-size: 117%;
	line-height: 112%;
	/* color: azure; */
	/* font-weight: bold; */
}

.Od3 h2, .Od3 h4, .Od3 ul, .Od3 li, .Od3 p {
	font-style: normal;
	margin: 0;
	padding: 0;
	border: none;
	font-size: 100%;
	line-height: 110%;
}

.Od3 ul {
	list-style-type: none
}

.Od3 li {
	list-style: none
}

.Od3 em {
	font-style: italic;
	/*font-family: Lora, ODESerif, serif;*/
}

.Od3 a {
	color: inherit;
	text-decoration: none;
	border-bottom: 1px dotted;
}

.dwy a {
	border-bottom: none;
}

/*.Od3 .xv4 a{color:#003fd2;text-decoration:none}*/

.Od3 a:hover {
	text-decoration: none;
	color: #6DBAEE
}

.k0i+.k0i {
	margin-top: 40px;
}

.b6i h4 {
	margin-bottom: 1em
}

.h1s {
	border-top: 1px solid #00bdf2;
	border-bottom: 1px solid #00bdf2;
	padding: 10px 0;
	line-height: 150%;
	position: relative;
}

h2.z2h, h2.hxy, .b6i h4 {
	display: inline-block;
	font-size: 1.5em;
	color: #1681c2;
	font-weight: bold;
	margin: 0;
}

h1s:first-child h2.hxy, h1s:first-child h2.z2h {
	margin-top: 0;
}

.tfr:before {
	content: "|"
}

.nah:before {
	content: "\0A6"
}

.sih:before {
	content: "\0B7"
}

.f0t .ysl {
	display: inline-block
}

.f0t .pxt {
	font-size: 90%
}

.f0t .b6i h4 {
	font-size: 100%;
	color: black;
	margin-bottom: 0
}

.f0t .b6i:before {
	content: "-";
	color: black;
	padding-right: 2px;
	position: absolute;
	left: -1em
}

.f0t .b6i {
	position: relative;
	margin-left: 1em
}

.f0t .b6i h4:after {
	content: ",";
	color: black;
	font-weight: normal;
	font-size: 90%
}

h2.z2h span, h2.hxy span {
	font-weight: normal;
	font-size: 95%
}

.pxt, .p2h {
	font-family: Gentium Plus, SansPhon, noto sans, arial, sans-serif;
	color: black;
	white-space: nowrap
}

.a8e {
	cursor: pointer;
	height: 1em;
	vertical-align: middle;
	position: relative;
}

h2.z2h .lx6, h2.hxy .lx6 {
	font-size: 50%;
	font-weight: normal;
	position: relative;
	vertical-align: super;
	padding-left: 2px
}

.k0z+.k0z {
	margin-top: 0.6em
}

h2.nvt {
	display: inline-block;
	font-weight: normal
}

.xno {
	display: inline-block;
	color: #f15a24;
	font-style: italic;
}

.nvt .xno {
	text-transform: uppercase;
	font-style: normal;
	font-weight: bold;
}

.cw6, .mbw a {
	font-weight: bold
}

.nvt {
	display: block;
	margin: 10px 0 5px 0;
}

.xno+.pzg {
	padding-left: 0.3em
}

.pzg {
	color: #333;
}

.rlx {
	font-style: normal;
	font-size: 100%;
	/* font-weight: bold; */
}

.iko {
	color: black;
	font-size: 100%;
	font-weight: bold;
}

em.tb0 {
	font-style: normal;
	color: #4b7aad;
	font-weight: bold;
	font-style: italic;
	/* font-family: Gentium Plus, SansPhon, Lora, ODESerif, serif; */
}

.u2n, .Od3 .se2 .u2n {
	margin: 0.2em 0 0 1em;
	position: relative
}

.Od3 .se2 {
	margin: 0 0 1em 0;
}

.ewq {
	margin: 0.5em 0 0 1em;
	position: relative;
	padding-left: 1.5em;
}

.ulk {}



.ld9 {}

.eh8 {}

.p9h {}

.dwy {}

.ld9+.aw5, .pzw+.aw5 {
	display: block;
}

em.xv4, li.lmn, .uxu em {
	font-style: italic;
	color: black;
	font-family: Lora, ODESerif, serif;
}

em.xv4, uxu em {}

li.lmn+li.lmn {
	font-size: 102%;
	rder-top: 1px solid black;
}

em.xv4 b, .uxu em b {
	/*font-size:100%*/
}

.eh8 em.xv4::before, .eh8 em.xv4::after {
	content: "'";
	font-size: 1rem;
}

.sdh {
	display: inline-block;
	cursor: pointer;
}

.sdh::before {
	content: "SYNONYMS";
	font-weight: bold;
	background: lightblue;
	color: white;
}

.sdh[show]::before {
	content: "SYNONYMS";
}

.x3z {
	display: inline-block;
	margin-right: 0.5em;
	cursor: pointer;
}

.xxn+.x3z::before {
	content: "+ examples";
}

.x3z::before {
	content: "+ examples";
}

.x3z::before::before {
	content: "";
	display: block;
}

.x3z+.ld9, .x3z+ul.rpz, .sdh+.pzw {
	display: none;
}

.xxn+.x3z[show]::before {
	content: "+ examples";
}

.x3z[show]::before {
	content: "+ examples";
}

.x3z[show]+.ld9, .x3z[show]+ul.rpz, .sdh[show]+.pzw {
	display: block;
}

.x3z::before, .sdh::before {
	display: inline-block;
	padding: 0 0.5em;
	border: 1px solid #2196F3;
	border-radius: 99em;
	/* text-decoration-line: underline; */
	/* background: #ffc10775; */
	/* color: #f15a24; */
	font-size: 85%;
	font-style: italic;
	/* font-weight: bold; */
	/* font-style: italic; */
}

.x3z[show]::before, .sdh[show]::before {
	color: white;
	background: #00bdf2;
	border: #666;
	/* content: "SYNONYMS"; */
	/* content: "+ examples"; */
}

.xxn, li.lmn {
	/* display: block; */
	position: relative;
	padding: 0.2em 0 0.2em 1em;
	font-size: 102%;
}

.xxn:before, .lmn:before {
	content: "•";
	display: inline-block;
	width: 1em;
	margin-left: -1em;
	text-align: center;
	color: #888;
	font-family: Open Sans, ODESans, sans-serif;
}

.pzw {
	padding-left: 1em;
	/* margin-left: 0.3em; */
	font-size: 100%;
	text-indent: -1em;
	/* line-height: 115%; */
	font-style: italic;
	/* border-left: 3px solid #dbdee2; */
}

ul.dhk, ul.rpz, .pzw {
	display: block;
	/*border-left: 3px solid #DDD;*/
}

.rnr, em.u0f, .cvq, .ix9, .m7g em {
	font-style: normal;
	color: #27a058;
	font-size: 100%;
	font-weight: bold;
	font-style: italic;
	font-family: Lora, ODESerif, serif;
}

.rnr {
	color: #e3533a;
}

/*.pzg .rnr, .pzg em.u0f, .pzg .cvq{font-size:90%}*/

.vkq {
	display: inline-block;
	color: black;
	min-width: 1em;
	text-align: right;
	margin-left: -1.5em;
	margin-right: 0.5em;
	font-size: 0.95em;
}

.ewq .vkq {
	font-size: 0.8em;
	min-width: 1.2em;
	text-align: left;
	display: inline-block;
	margin-right: 0.3em;
	margin-left: -2em;
	font-family: Open Sans Condensed Light, ODECondensed, sans-serif
}

.qbl {
	color: black;
	font-size: 100%
}

.b9e {
	/*color:#930;*/
}

.b9e:after {
	content: "."
}

div.uxu div.ysl p b.b9e~b.b9e::before {
	content: "";
	display: block;
	height: 0.5em;
}

.e8l .q5j, .pdj {
	color: black;
	font-weight: bold;
}

.s0c, .f0t, .m7g, .uxu, .e8l {
	margin-top: 0.5em;
	clear: both
}

.tki {
	font-weight: bold;
	color: #6DBAEE;
	font-size: 1.1em;
}

.s0c h2, .f0t h2, .m7g h2, .uxu h2, .e8l h2 {
	border-top: 1px solid #00bdf2;
	margin-bottom: .3em;
	margin-top: 1em;
}

.sgx {
	font-variant: small-caps;
	font-size: 100%
}

.s0c p:before, .n3h:before, .mbw:before {
	content: "\021E8\020"
}

.rqo {
	color: black;
	font-size: 90%;
	font-weight: normal
}

h2.z2h .l6p, h2.hxy .l6p, h4 .l6p, .rqo .l6p {
	color: black;
	font-size: 110%;
	font-weight: bold
}

.f0t .l6p {
	color: black
}

.n3h, .mbw {
	display: block;
	padding-top: 0.8em
}

.aej, .yuq {
	float: right;
	width: 13px;
	height: 6px;
	cursor: pointer;
	position: relative;
	top: 1px;
	padding: .3em .1em .3em .3em
}

.yuq {
	transform: scaleY(-1);
	-webkit-transform: scaleY(-1);
	filter: FlipV
}

.dzg, .ynx, .eju, ul.s6x, .e8l .dhk {
	color: #555;
	border-left: 3px solid #00bdf2;
	margin: 0.5em 0 0.3em;
	padding-left: .5em
}

.dzg, .ynx {
	display: block;
	margin-left: 1em
}

.e8l .dhk {
	display: block
}

.j02, .g4p {
	margin: 0 1ex 1ex 1ex;
	position: relative;
	z-index: 999;
	float: right;
	clear: right;
}

.j02 {
	width: 40%!important;
	height: auto;
}

.g4p {
	width: 99%;
	height: auto;
}

.Od3 img[onclick] {
	cursor: pointer
}

.s0c p:before {
	content: "•";
	color: #555;
	font-family: Lora, ODESerif, serif;
	margin-right: 0.5em;
}

.s0c p {
	display: block;
	position: relative;
	margin-left: 0.2em;
}

.mla {
	clear: both
}

.h1s .pxt {
	position: relative;
	z-index: 2;
	display: inline-block;
}

.h1s .pxt::before {
	content: '';
	height: 1.5rem;
	display: inline-block;
}

.cn_def {
	/* font-family: Open Sans, ODESans, sans-serif; */
	font-size: 90%;
	display: block;
	padding: 0 0 0 .5em;
	/* font-weight: bold; */
}
.aw5+.cn_def {
	display: inline;
}
p.cn {
	/* font-style: italic; */
	color: #888;
	font-style: normal;
	font-size: 70%;
	display: inline;
	position: relative;
	padding: 0 0 0 0.5em;
}
.pxt a{
border-bottom: none;
}
`

const oald9css = `@font-face{font-family:'oalecd9';src:url("oalecd9.ttf");font-weight:400;font-style:normal}
body{background-color:#fffefe;font-family:'oalecd9';counter-reset:sn_blk_counter}
.cixing_part{counter-reset:sn_blk_counter}
.cixing_tiaozhuan_part{display:inline;color:#c70000}
.cixing_tiaozhuan_part a:link{text-decoration:none;font-weight:600}
.cixing_tiaozhuan_part a{color:#c70000}
h{font-weight:600;color:#323270;font-size:22px}
boxtag{font-size:13px;font-weight:600;border-style:solid;color:#fff;background-color:blue;border-color:blue;border-width:1px;margin-top:2px;padding-left:2px;padding-right:2px;border-radius:10px}
boxtag[type="awl"]{font-size:9px;font-weight:600;color:#fff;border-style:solid;border-width:1px;background-color:#000;border-color:#000;padding-left:1px;padding-right:1px;border-top:0;border-bottom:0}
vp-gs{display:none}
pron-g-blk{display:inline}
top-g{display:block}
pron-g-blk brelabel{padding-left:4px;font-size:14px}
pron-g-blk namelabel{padding-left:4px;font-size:14px}
pos xhtml\:a{display:table;color:#fff;font-weight:600;padding-left:2px;padding-right:2px;border-style:solid;border-width:1px;border-radius:5px;border-top:0;border-bottom:0;border-color:#c70000;background-color:#c70000}
vpform{color:#9b9b9b;font-style:italic}
vp-g{display:block;padding-left:12px}
sn-blk{display:block}
:not(idm-g) sn-gs sn-blk::before{padding-right:4px;counter-increment:sn_blk_counter;content:counter(sn_blk_counter)}
:not(id-g) sn-gs sn-blk::before{padding-right:4px;counter-increment:sn_blk_counter;content:counter(sn_blk_counter)}
def{font-weight:600}
xsymb{display:none}
xhtml\:br{}
x-g-blk{display:block;border-left:3px solid #dbdbdb;margin-left:8px;padding-left:10px}
x-g-blk x::before{content:'•'}
x-g-blk x{font-style:italic;color:#3784dd}
x-g-blk x chn{padding-left:13px;font-style:normal;color:#8d8d8d}
top-g xhtml\:br{display:none}
cf-blk{font-style:italic;font-weight:600;color:#2b7dca;padding-right:4px}
xr-gs{display:block}
xr-g-blk a:link{text-decoration:none;color:#a52a2a;font-weight:600}
def+x-gs cf-blk{font-style:italic;font-weight:600;color:#2b7dca;display:block}
shcut-blk{margin-top:14px;display:block;border-bottom:1px solid #a0a0a0;padding-bottom:5px}
gram-g{font-weight:600;color:#04b92b}
unbox{margin-top:16px;margin-bottom:16px;display:block;padding-left:5px;padding-right:15px;padding-top:10px;border:1px solid red;border-radius:12px}
unbox title{display:none}
unbox inlinelist{display:inline}
unbox inlinelist und{font-weight:600;color:#03648a}
unbox unsyn{display:block;font-weight:600;color:#1a4781}
unbox x-g-blk{display:block}
unbox x-g-blk x::before{content:'•';padding-right:6px}
unbox h3{color:#36866a;margin-bottom:4px;margin-top:6px}
unbox eb{font-weight:600}
pron-g-blk a:link{text-decoration:none}
audio-gbs-liju,audio-gb-liju,audio-brs-liju,audio-gb{padding-right:4px;color:blue;opacity:.8;display:none}
audio-uss-liju,audio-ams-liju,audio-us-liju,audio-us{padding-right:4px;color:#af0404;opacity:.8;display:none}
a:link{text-decoration:none}
eb{font-weight:600}
idm-gs un{display:block;color:#7c7070}
idm-blk idm{padding-top:12px;display:block;font-weight:600;color:#010102}
idm-g def{font-weight:500}
idm-g sn-blk::before{color:#6f49c7;content:'★'}
pv-g def{font-weight:500}
label-g-blk{color:#797979;font-style:italic}
pv-blk pv{padding-top:12px;display:block;font-weight:600;color:#1881e4}
unbox ul li{list-style-type:square}
unbox x-gs{display:block;margin-left:8px;padding-left:10px}
unbox x-gs chn{}
img{display:block;max-width:100%}
.big_pic{display:none;max-width:100%}
.switch_ec{display:none}
if-gs-blk{display:inline}
if-gs-blk form{display:inline}
unbox[type=wordfinder] xr-gs{display:inline}
unbox[type=wordfinder]::before{border:1px solid;border-radius:6px;position:relative;top:-24px;font-size:18px;color:#af1919;font-weight:600;content:'WordFinder';background-color:#fff;padding:5px 7px}
unbox[type=colloc]::before{border:1px solid;border-radius:6px;position:relative;top:-24px;left:18px;font-size:18px;color:#af1919;font-weight:600;content:'Collocations 词语搭配';background-color:#fff;padding:5px 7px}
unbox[type=wordfamily]{display:block;float:right}
unbox[type=wordfamily] wfw-g{display:block}
unbox[type=wordfamily] wfw-g wfw-blk{color:#101095;font-weight:600}
unbox[type=wordfamily] wfw-g wfo{font-weight:600}
unbox[type=wordfamily] wfw-g wfp-blk wfp{font-style:italic;color:#971717;font-weight:500}
unbox[type=wordfamily]::before{border:1px solid;border-radius:6px;position:relative;top:-24px;left:18px;font-size:18px;color:#af1919;font-weight:600;content:'WORD FAMILY';background-color:#fff;padding:5px 7px}
unbox[type=grammar]::before{border:1px solid;border-radius:6px;position:relative;top:-24px;left:18px;font-size:18px;color:#af1919;font-weight:600;content:'GRAMMAR 语法';background-color:#fff;padding:5px 7px}
unbox[type=grammar]{margin-top:36px}
unbox[type=grammar] x-gs{padding-left:0;margin-left:0}
unbox ul{margin-top:4px}
use-blk{color:#0b8a0b}
dis-g xr-gs{display:inline}
xr-gs[firstinblock="n"]{display:inline}
`

func ParseAddr(listen string) (network string, address string) {
	// Allow passing just -remote=auto, as a shorthand for using automatic remote
	// resolution.
	if listen == "auto" {
		return "auto", ""
	}
	if parts := strings.SplitN(listen, ";", 2); len(parts) == 2 {
		return parts[0], parts[1]
	}
	return "tcp", listen
}
