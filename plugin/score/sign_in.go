// Package score 签到，答题得分
package score

import (
	"fmt"
	"math"
	"math/rand"
	"os"
	"strconv"
	"time"

	"github.com/FloatTech/AnimeAPI/bilibili"
	"github.com/FloatTech/AnimeAPI/wallet"
	"github.com/FloatTech/floatbox/file"
	"github.com/FloatTech/floatbox/process"
	"github.com/FloatTech/floatbox/web"
	"github.com/FloatTech/gg"
	"github.com/FloatTech/imgfactory"
	ctrl "github.com/FloatTech/zbpctrl"
	"github.com/FloatTech/zbputils/control"
	"github.com/FloatTech/zbputils/ctxext"
	"github.com/FloatTech/zbputils/img/text"
	"github.com/golang/freetype"
	log "github.com/sirupsen/logrus"
	"github.com/wcharczuk/go-chart/v2"
	zero "github.com/wdvxdr1123/ZeroBot"
	"github.com/wdvxdr1123/ZeroBot/message"
)

const (
	backgroundURL = "https://iw233.cn/api.php?sort=pc"
	referer       = "https://weibo.com/"
	signinMax     = 1
	// SCOREMAX 分数上限定为1200
	SCOREMAX = 1200
)

var (
	rankArray = [...]int{0, 10, 20, 50, 100, 200, 350, 550, 750, 1000, 1200}
	engine    = control.Register("score", &ctrl.Options[*zero.Ctx]{
		DisableOnDefault:  false,
		Brief:             "签到",
		Help:              "- 签到\n- 获得签到背景[@xxx] | 获得签到背景\n- 查看等级排名\n注:为跨群排名\n- 查看我的钱包\n- 查看钱包排名\n注:为本群排行，若群人数太多不建议使用该功能!!!",
		PrivateDataFolder: "score",
	})
)

func init() {
	cachePath := engine.DataFolder() + "cache/"
	go func() {
		_ = os.RemoveAll(cachePath)
		err := os.MkdirAll(cachePath, 0755)
		if err != nil {
			panic(err)
		}
		sdb = initialize(engine.DataFolder() + "score.db")
		mdb = initializeBJ(engine.DataFolder() + "bj.db")
	}()

		

	engine.OnFullMatch("特权",zero.AdminPermission).Limit(ctxext.LimitByUser).SetBlock(true).
		Handle(func(ctx *zero.Ctx) {
			uid := ctx.Event.UserID
			// 签到图片
			add := 2992 // 等级越高获得的钱越高
			err := wallet.InsertWalletOf(uid, add)
			if err != nil {
				ctx.SendChain(message.Text("ERROR: ", err))
				return
			}
			ctx.SendChain(message.Text("充值成功: "))
		})

	engine.OnRegex(`^查看盒狗(\[CQ:at,qq=(\d+)\]\s?|(\d+))`, zero.AdminPermission).Limit(ctxext.LimitByUser).SetBlock(true).
	Handle(func(ctx *zero.Ctx) {
		fid := ctx.State["regex_matched"].([]string)
		fiancee, _ := strconv.ParseInt(fid[2]+fid[3], 10, 64)
		score := mdb.GetBJScoreByUID(fiancee).Score
		ctx.SendChain(message.Text(strconv.FormatInt(fiancee,10)+ " 盒狗分: ",score))
	})

	engine.OnRegex(`^标记盒狗(\[CQ:at,qq=(\d+)\]\s?|(\d+))`, zero.AdminPermission).Limit(ctxext.LimitByUser).SetBlock(true).
	Handle(func(ctx *zero.Ctx) {
		fid := ctx.State["regex_matched"].([]string)
		fiancee, _ := strconv.ParseInt(fid[2]+fid[3], 10, 64)
		score := mdb.GetBJScoreByUID(fiancee).Score + 1
		mdb.InsertOrUpdateBJScoreByUID(fiancee,score)
		ctx.SendChain(message.Text(strconv.FormatInt(fiancee,10)+ " 盒狗分: ",score))
		duration := 8*60*score
		if duration >= 43200 {
			duration = 43199 // qq禁言最大时长为一个月
		}
		ctx.SetGroupBan(
			ctx.Event.GroupID,
			fiancee, // 要禁言的人的qq
			int64(duration*60), // 要禁言的时间（分钟）
		)
		ctx.SendChain(message.Text("小黑屋收留成功~"))
	})

	engine.OnRegex(`^清零分数(\[CQ:at,qq=(\d+)\]\s?|(\d+))`, zero.AdminPermission).Limit(ctxext.LimitByUser).SetBlock(true).
	Handle(func(ctx *zero.Ctx) {
		fid := ctx.State["regex_matched"].([]string)
		fiancee, _ := strconv.ParseInt(fid[2]+fid[3], 10, 64)
		mdb.InsertOrUpdateBJScoreByUID(fiancee,0)
		ctx.SendChain(message.Text("已清零"))
	})

	engine.OnFullMatch("查看盒狗排名", zero.OnlyGroup,zero.AdminPermission).Limit(ctxext.LimitByGroup).SetBlock(true).
		Handle(func(ctx *zero.Ctx) {
			today := time.Now().Format("20060102 11:11:11")
			drawedFile := cachePath + today + "hgscoreRank.png"
			st, err := mdb.GetBJScoreRankByTopN(10)
			if err != nil {
				ctx.SendChain(message.Text("ERROR: ", err))
				return
			}
			if len(st) == 0 {
				ctx.SendChain(message.Text("ERROR: 目前还没有盒狗被标记"))
				return
			}
			_, err = file.GetLazyData(text.FontFile, control.Md5File, true)
			if err != nil {
				ctx.SendChain(message.Text("ERROR: ", err))
				return
			}
			b, err := os.ReadFile(text.FontFile)
			if err != nil {
				ctx.SendChain(message.Text("ERROR: ", err))
				return
			}
			font, err := freetype.ParseFont(b)
			if err != nil {
				ctx.SendChain(message.Text("ERROR: ", err))
				return
			}
			f, err := os.Create(drawedFile)
			if err != nil {
				ctx.SendChain(message.Text("ERROR: ", err))
				return
			}
			var bars []chart.Value
			for _, v := range st {
				if v.Score != 0 {
					bars = append(bars, chart.Value{
						Label: ctx.CardOrNickName(v.UID),
						Value: float64(v.Score),
					})
				}
			}
			err = chart.BarChart{
				Font:  font,
				Title: "盒狗TOP10",
				Background: chart.Style{
					Padding: chart.Box{
						Top: 40,
					},
				},
				YAxis: chart.YAxis{
					Range: &chart.ContinuousRange{
						Min: 0,
						Max: math.Ceil(bars[0].Value/10) * 10,
					},
				},
				Height:   500,
				BarWidth: 50,
				Bars:     bars,
			}.Render(chart.PNG, f)
			_ = f.Close()
			if err != nil {
				_ = os.Remove(drawedFile)
				ctx.SendChain(message.Text("ERROR: ", err))
				return
			}
			ctx.SendChain(message.Image("file:///" + file.BOTPATH + "/" + drawedFile))
		})
	

	//zero.OnlyGroup, zero.AdminPermission
	engine.OnFullMatch("签到").Limit(ctxext.LimitByUser).SetBlock(true).
		Handle(func(ctx *zero.Ctx) {
			uid := ctx.Event.UserID
			now := time.Now()
			today := now.Format("20060102")
			// 签到图片
			drawedFile := cachePath + strconv.FormatInt(uid, 10) + today + "signin.png"
			picFile := cachePath + strconv.FormatInt(uid, 10) + today + ".png"
			// 获取签到时间
			si := sdb.GetSignInByUID(uid)
			siUpdateTimeStr := si.UpdatedAt.Format("20060102")
			switch {
			case si.Count >= signinMax && siUpdateTimeStr == today:
				// 如果签到时间是今天
				ctx.SendChain(message.Reply(ctx.Event.MessageID), message.Text("今天你已经签到过了！"))
				if file.IsExist(drawedFile) {
					ctx.SendChain(message.Image("file:///" + file.BOTPATH + "/" + drawedFile))
				}
				return
			case siUpdateTimeStr != today:
				// 如果是跨天签到就清数据
				err := sdb.InsertOrUpdateSignInCountByUID(uid, 0)
				if err != nil {
					ctx.SendChain(message.Text("ERROR: ", err))
					return
				}
			}
			// 更新签到次数
			err := sdb.InsertOrUpdateSignInCountByUID(uid, si.Count+1)
			if err != nil {
				ctx.SendChain(message.Text("ERROR: ", err))
				return
			}
			// 更新经验
			level := sdb.GetScoreByUID(uid).Score + 1
			if level > SCOREMAX {
				level = SCOREMAX
				ctx.SendChain(message.At(uid), message.Text("你的等级已经达到上限"))
			}
			err = sdb.InsertOrUpdateScoreByUID(uid, level)
			if err != nil {
				ctx.SendChain(message.Text("ERROR: ", err))
				return
			}
			// 更新钱包
			rank := getrank(level)
			add := 1 + rand.Intn(10) + rank*5 // 等级越高获得的钱越高
			err = wallet.InsertWalletOf(uid, add)
			if err != nil {
				ctx.SendChain(message.Text("ERROR: ", err))
				return
			}
			score := wallet.GetWalletOf(uid)
			// 绘图
			err = initPic(picFile)
			if err != nil {
				ctx.SendChain(message.Text("ERROR: ", err))
				return
			}
			back, err := gg.LoadImage(picFile)
			if err != nil {
				ctx.SendChain(message.Text("ERROR: ", err))
				return
			}
			// 避免图片过大，最大 1280*720
			back = imgfactory.Limit(back, 1280, 720)
			canvas := gg.NewContext(back.Bounds().Size().X, int(float64(back.Bounds().Size().Y)*1.7))
			canvas.SetRGB(1, 1, 1)
			canvas.Clear()
			canvas.DrawImage(back, 0, 0)
			monthWord := now.Format("01/02")
			hourWord := getHourWord(now)
			data, err := file.GetLazyData(text.BoldFontFile, control.Md5File, true)
			if err != nil {
				ctx.SendChain(message.Text("ERROR: ", err))
				return
			}
			if err = canvas.ParseFontFace(data, float64(back.Bounds().Size().X)*0.1); err != nil {
				ctx.SendChain(message.Text("ERROR: ", err))
				return
			}
			canvas.SetRGB(0, 0, 0)
			canvas.DrawString(hourWord, float64(back.Bounds().Size().X)*0.1, float64(back.Bounds().Size().Y)*1.2)
			canvas.DrawString(monthWord, float64(back.Bounds().Size().X)*0.6, float64(back.Bounds().Size().Y)*1.2)
			nickName := ctx.CardOrNickName(uid)
			data, err = file.GetLazyData(text.FontFile, control.Md5File, true)
			if err != nil {
				ctx.SendChain(message.Text("ERROR: ", err))
				return
			}
			if err = canvas.ParseFontFace(data, float64(back.Bounds().Size().X)*0.04); err != nil {
				ctx.SendChain(message.Text("ERROR: ", err))
				return
			}
			canvas.DrawString(nickName+fmt.Sprintf(" ATRI币+%d", add), float64(back.Bounds().Size().X)*0.1, float64(back.Bounds().Size().Y)*1.3)
			canvas.DrawString("当前ATRI币:"+strconv.FormatInt(int64(score), 10), float64(back.Bounds().Size().X)*0.1, float64(back.Bounds().Size().Y)*1.4)
			canvas.DrawString("LEVEL:"+strconv.FormatInt(int64(rank), 10), float64(back.Bounds().Size().X)*0.1, float64(back.Bounds().Size().Y)*1.5)
			canvas.DrawRectangle(float64(back.Bounds().Size().X)*0.1, float64(back.Bounds().Size().Y)*1.55, float64(back.Bounds().Size().X)*0.6, float64(back.Bounds().Size().Y)*0.1)
			canvas.SetRGB255(150, 150, 150)
			canvas.Fill()
			var nextrankScore int
			if rank < 10 {
				nextrankScore = rankArray[rank+1]
			} else {
				nextrankScore = SCOREMAX
			}
			canvas.SetRGB255(0, 0, 0)
			canvas.DrawRectangle(float64(back.Bounds().Size().X)*0.1, float64(back.Bounds().Size().Y)*1.55, float64(back.Bounds().Size().X)*0.6*float64(level)/float64(nextrankScore), float64(back.Bounds().Size().Y)*0.1)
			canvas.SetRGB255(102, 102, 102)
			canvas.Fill()
			canvas.DrawString(fmt.Sprintf("%d/%d", level, nextrankScore), float64(back.Bounds().Size().X)*0.75, float64(back.Bounds().Size().Y)*1.62)

			f, err := os.Create(drawedFile)
			if err != nil {
				log.Errorln("[score]", err)
				data, err := imgfactory.ToBytes(canvas.Image())
				if err != nil {
					log.Errorln("[score]", err)
					return
				}
				ctx.SendChain(message.ImageBytes(data))
				return
			}
			_, err = imgfactory.WriteTo(canvas.Image(), f)
			_ = f.Close()
			if err != nil {
				ctx.SendChain(message.Text("ERROR: ", err))
				return
			}
			ctx.SendChain(message.Image("file:///" + file.BOTPATH + "/" + drawedFile))
		})
	engine.OnPrefix("获得签到背景", zero.OnlyGroup).Limit(ctxext.LimitByGroup).SetBlock(true).
		Handle(func(ctx *zero.Ctx) {
			param := ctx.State["args"].(string)
			var uidStr string
			if len(ctx.Event.Message) > 1 && ctx.Event.Message[1].Type == "at" {
				uidStr = ctx.Event.Message[1].Data["qq"]
			} else if param == "" {
				uidStr = strconv.FormatInt(ctx.Event.UserID, 10)
			}
			picFile := cachePath + uidStr + time.Now().Format("20060102") + ".png"
			if file.IsNotExist(picFile) {
				ctx.SendChain(message.Reply(ctx.Event.MessageID), message.Text("请先签到！"))
				return
			}
			ctx.SendChain(message.Image("file:///" + file.BOTPATH + "/" + picFile))
		})
	engine.OnFullMatch("查看等级排名", zero.OnlyGroup).Limit(ctxext.LimitByGroup).SetBlock(true).
		Handle(func(ctx *zero.Ctx) {
			today := time.Now().Format("20060102")
			drawedFile := cachePath + today + "scoreRank.png"
			if file.IsExist(drawedFile) {
				ctx.SendChain(message.Image("file:///" + file.BOTPATH + "/" + drawedFile))
				return
			}
			st, err := sdb.GetScoreRankByTopN(10)
			if err != nil {
				ctx.SendChain(message.Text("ERROR: ", err))
				return
			}
			if len(st) == 0 {
				ctx.SendChain(message.Text("ERROR: 目前还没有人签到过"))
				return
			}
			_, err = file.GetLazyData(text.FontFile, control.Md5File, true)
			if err != nil {
				ctx.SendChain(message.Text("ERROR: ", err))
				return
			}
			b, err := os.ReadFile(text.FontFile)
			if err != nil {
				ctx.SendChain(message.Text("ERROR: ", err))
				return
			}
			font, err := freetype.ParseFont(b)
			if err != nil {
				ctx.SendChain(message.Text("ERROR: ", err))
				return
			}
			f, err := os.Create(drawedFile)
			if err != nil {
				ctx.SendChain(message.Text("ERROR: ", err))
				return
			}
			var bars []chart.Value
			for _, v := range st {
				if v.Score != 0 {
					bars = append(bars, chart.Value{
						Label: ctx.CardOrNickName(v.UID),
						Value: float64(v.Score),
					})
				}
			}
			err = chart.BarChart{
				Font:  font,
				Title: "等级排名(1天只刷新1次)",
				Background: chart.Style{
					Padding: chart.Box{
						Top: 40,
					},
				},
				YAxis: chart.YAxis{
					Range: &chart.ContinuousRange{
						Min: 0,
						Max: math.Ceil(bars[0].Value/10) * 10,
					},
				},
				Height:   500,
				BarWidth: 50,
				Bars:     bars,
			}.Render(chart.PNG, f)
			_ = f.Close()
			if err != nil {
				_ = os.Remove(drawedFile)
				ctx.SendChain(message.Text("ERROR: ", err))
				return
			}
			ctx.SendChain(message.Image("file:///" + file.BOTPATH + "/" + drawedFile))
		})
}

func getHourWord(t time.Time) string {
	h := t.Hour()
	switch {
	case 6 <= h && h < 12:
		return "早上好"
	case 12 <= h && h < 14:
		return "中午好"
	case 14 <= h && h < 19:
		return "下午好"
	case 19 <= h && h < 24:
		return "晚上好"
	case 0 <= h && h < 6:
		return "凌晨好"
	default:
		return ""
	}
}

func getrank(count int) int {
	for k, v := range rankArray {
		if count == v {
			return k
		} else if count < v {
			return k - 1
		}
	}
	return -1
}

func initPic(picFile string) error {
	if file.IsExist(picFile) {
		return nil
	}
	defer process.SleepAbout1sTo2s()
	url, err := bilibili.GetRealURL(backgroundURL)
	if err != nil {
		return err
	}
	data, err := web.RequestDataWith(web.NewDefaultClient(), url, "", referer, "", nil)
	if err != nil {
		return err
	}
	return os.WriteFile(picFile, data, 0644)
}
