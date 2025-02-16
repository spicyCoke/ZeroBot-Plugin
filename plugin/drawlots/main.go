// Package drawlots 多功能抽签插件
package drawlots

import (
	"errors"
	"image"
	"image/gif"
	"math/rand"
	"os"
	"strconv"
	"strings"

	"github.com/FloatTech/floatbox/file"
	"github.com/FloatTech/imgfactory"
	ctrl "github.com/FloatTech/zbpctrl"
	control "github.com/FloatTech/zbputils/control"
	"github.com/FloatTech/zbputils/ctxext"
	"github.com/FloatTech/zbputils/img/text"
	"github.com/fumiama/jieba/util/helper"
	"github.com/sirupsen/logrus"
	zero "github.com/wdvxdr1123/ZeroBot"
	"github.com/wdvxdr1123/ZeroBot/message"
)

type info struct {
	lotsType string // 文件后缀
	quantity int    // 签数
}

var (
	lotsList = func() map[string]info {
		lotsList, err := getList()
		if err != nil {
			logrus.Infoln("[drawlots]加载失败:", err)
		} else {
			logrus.Infoln("[drawlots]加载", len(lotsList), "个抽签")
		}
		return lotsList
	}()
	en = control.Register("drawlots", &ctrl.Options[*zero.Ctx]{
		DisableOnDefault:  false,
		Brief:             "多功能抽签",
		Help:              "支持图包文件夹和gif抽签\n-------------\n- (刷新)抽签列表\n- 抽[签名]签\n- 看签[gif签名]\n- 加签[签名][gif图片]\n- 删签[gif签名]",
		PrivateDataFolder: "drawlots",
	}).ApplySingle(ctxext.DefaultSingle)
	datapath = file.BOTPATH + "/" + en.DataFolder()
)

func init() {
	en.OnFullMatchGroup([]string{"抽签列表", "刷新抽签列表"}).SetBlock(true).Handle(func(ctx *zero.Ctx) {
		lotsList, err := getList() // 刷新列表
		if err != nil {
			ctx.SendChain(message.Text("ERROR:", err))
			return
		}
		messageText := &strings.Builder{}
		messageText.WriteString(" 签 名 [ 类 型 ]----签数\n")
		messageText.WriteString("———————————\n")
		for name, fileInfo := range lotsList {
			messageText.WriteString(name + "[" + fileInfo.lotsType + "]----" + strconv.Itoa(fileInfo.quantity) + "\n")
			messageText.WriteString("----------\n")
		}
		textPic, err := text.RenderToBase64(messageText.String(), text.BoldFontFile, 400, 50)
		if err != nil {
			ctx.SendChain(message.Text("ERROR:", err))
			return
		}
		ctx.SendChain(message.Image("base64://" + helper.BytesToString(textPic)))
	})
	en.OnRegex(`^抽(.+)签$`).SetBlock(true).Handle(func(ctx *zero.Ctx) {
		lotsType := ctx.State["regex_matched"].([]string)[1]
		fileInfo, ok := lotsList[lotsType]
		if !ok {
			ctx.SendChain(message.Text("签名[", lotsType, "]不存在"))
			return
		}
		if fileInfo.lotsType == "folder" {
			picPath, err := randFile(lotsType, 3)
			if err != nil {
				ctx.SendChain(message.Text("ERROR:", err))
				return
			}
			ctx.SendChain(message.Reply(ctx.Event.MessageID), message.Image("file:///"+picPath))
			return
		}
		lotsImg, err := randGif(lotsType + "." + fileInfo.lotsType)
		if err != nil {
			ctx.SendChain(message.Text("ERROR:", err))
			return
		}
		// 生成图片
		data, err := imgfactory.ToBytes(lotsImg)
		if err != nil {
			ctx.SendChain(message.Text("ERROR:", err))
			return
		}
		ctx.SendChain(message.Reply(ctx.Event.MessageID), message.ImageBytes(data))
	})
	en.OnPrefix("看签", zero.UserOrGrpAdmin).SetBlock(true).Limit(ctxext.LimitByUser).Handle(func(ctx *zero.Ctx) {
		id := ctx.Event.MessageID
		lotsName := strings.TrimSpace(ctx.State["args"].(string))
		fileInfo, ok := lotsList[lotsName]
		if !ok {
			ctx.Send(message.ReplyWithMessage(id, message.Text("签名[", lotsName, "]不存在")))
			return
		}
		if fileInfo.lotsType == "folder" {
			ctx.Send(message.ReplyWithMessage(id, message.Text("仅支持查看gif抽签")))
			return
		}
		ctx.Send(message.ReplyWithMessage(id, message.Image("file:///"+datapath+lotsName+"."+fileInfo.lotsType)))
	})
	en.OnPrefix("加签", zero.SuperUserPermission, zero.MustProvidePicture).SetBlock(true).Limit(ctxext.LimitByUser).Handle(func(ctx *zero.Ctx) {
		id := ctx.Event.MessageID
		lotsName := strings.TrimSpace(ctx.State["args"].(string))
		if lotsName == "" {
			ctx.Send(message.ReplyWithMessage(id, message.Text("请使用正确的指令形式")))
			return
		}
		picURL := ctx.State["image_url"].([]string)[0]
		err := file.DownloadTo(picURL, datapath+"/"+lotsName+".gif")
		if err != nil {
			ctx.SendChain(message.Text("ERROR:", err))
			return
		}
		file, err := os.Open(datapath + "/" + lotsName + ".gif")
		if err != nil {
			ctx.SendChain(message.Text("ERROR:", err))
			return
		}
		im, err := gif.DecodeAll(file)
		_ = file.Close()
		if err != nil {
			ctx.SendChain(message.Text("ERROR:", err))
			return
		}
		lotsList[lotsName] = info{
			lotsType: "gif",
			quantity: len(im.Image),
		}
		ctx.Send(message.ReplyWithMessage(id, message.Text("成功")))
	})
	en.OnPrefix("删签", zero.SuperUserPermission).SetBlock(true).Limit(ctxext.LimitByUser).Handle(func(ctx *zero.Ctx) {
		id := ctx.Event.MessageID
		lotsName := strings.TrimSpace(ctx.State["args"].(string))
		fileInfo, ok := lotsList[lotsName]
		if !ok {
			ctx.Send(message.ReplyWithMessage(id, message.Text("签名[", lotsName, "]不存在")))
			return
		}
		if fileInfo.lotsType == "folder" {
			ctx.Send(message.ReplyWithMessage(id, message.Text("图包请手动移除(保护图源误删),谢谢")))
			return
		}
		err := os.Remove(datapath + lotsName + "." + fileInfo.lotsType)
		if err != nil {
			ctx.SendChain(message.Text("ERROR:", err))
			return
		}
		delete(lotsList, lotsName)
		ctx.Send(message.ReplyWithMessage(id, message.Text("成功")))
	})
}

func getList() (list map[string]info, err error) {
	list = make(map[string]info, 100)
	files, err := os.ReadDir(datapath)
	if err != nil {
		return
	}
	if len(files) == 0 {
		err = errors.New("不存在任何抽签")
		return
	}
	for _, lots := range files {
		if lots.IsDir() {
			files, _ := os.ReadDir(datapath + "/" + lots.Name())
			list[lots.Name()] = info{
				lotsType: "folder",
				quantity: len(files),
			}
			continue
		}
		before, after, ok := strings.Cut(lots.Name(), ".")
		if !ok || before == "" {
			continue
		}
		file, err := os.Open(datapath + "/" + lots.Name())
		if err != nil {
			return nil, err
		}
		im, err := gif.DecodeAll(file)
		_ = file.Close()
		if err != nil {
			return nil, err
		}
		list[before] = info{
			lotsType: after,
			quantity: len(im.Image),
		}
	}
	return
}

func randFile(path string, indexMax int) (string, error) {
	picPath := datapath + path
	files, err := os.ReadDir(picPath)
	if err != nil {
		return "", err
	}
	if len(files) > 0 {
		drawFile := files[rand.Intn(len(files))]
		// 如果是文件夹就递归
		if drawFile.IsDir() {
			indexMax--
			if indexMax <= 0 {
				return "", errors.New("图包[" + path + "]存在太多非图片文件,请清理")
			}
			return randFile(path, indexMax)
		}
		return picPath + "/" + drawFile.Name(), err
	}
	return "", errors.New("图包[" + path + "]不存在签内容")
}

func randGif(gifName string) (image.Image, error) {
	name := datapath + gifName
	file, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	im, err := gif.DecodeAll(file)
	_ = file.Close()
	if err != nil {
		return nil, err
	}
	/*
		firstImg, err := imgfactory.Load(name)
		if err != nil {
			return nil, err
		}
		v := im.Image[rand.Intn(len(im.Image))]
		return imgfactory.Size(firstImg, firstImg.Bounds().Max.X, firstImg.Bounds().Max.Y).InsertUpC(v, 0, 0, firstImg.Bounds().Max.X/2, firstImg.Bounds().Max.Y/2).Clone().Image(),err
	/*/
	// 如果gif图片出现信息缺失请使用上面注释掉的代码，把下面注释了(上面代码部分图存在bug)
	v := im.Image[rand.Intn(len(im.Image))]
	return imgfactory.Size(v, v.Bounds().Max.X, v.Bounds().Max.Y).Image(), err
	// */
}
