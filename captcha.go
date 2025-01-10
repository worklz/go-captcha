package captcha

import (
	"bytes"
	"crypto/md5"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/golang/freetype"
	"github.com/golang/freetype/truetype"
	"github.com/gomodule/redigo/redis"
	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"
)

const (
	fontDir = "./fonts/"
)

type Captcha struct {
	// 验证码字符集合
	codeSet string
	// 验证码字体大小(px)
	fontSize int
	// 是否画混淆曲线
	useCurve bool
	// 是否添加杂点
	useNoise bool
	// 验证码位数
	length int
	// 背景颜色（红、绿、蓝）
	bg [3]int
	// 验证码图片高度
	imageH int
	// 验证码图片宽度
	imageW int
	// 字体
	font *truetype.Font
}

func NewCaptcha() *Captcha {
	rand.Seed(time.Now().UnixNano())
	return &Captcha{
		codeSet:  "2345678abcdefhijkmnpqrstuvwxyzABCDEFGHJKLMNPQRTUVWXY",
		fontSize: 29,
		useCurve: true,
		useNoise: true,
		length:   4,
		bg:       [3]int{243, 251, 254},
	}
}

func (c *Captcha) GenerateImageBase64() (string, string, error) {
	// 设置随机字体
	err := c.setRandomFont()
	if err != nil {
		return "", "", err
	}
	code := c.generateCode()

	// 图片宽(px)
	c.imageW = int(float64(c.length)*float64(c.fontSize)*1.5) + c.length*c.fontSize/2
	// 图片高(px)
	c.imageH = int(float64(c.fontSize) * 2.5)

	img := image.NewRGBA(image.Rect(0, 0, c.imageW, c.imageH))
	bgColor := color.RGBA{uint8(c.bg[0]), uint8(c.bg[1]), uint8(c.bg[2]), 255}
	draw.Draw(img, img.Bounds(), &image.Uniform{bgColor}, image.Point{}, draw.Src)

	face := truetype.NewFace(c.font, &truetype.Options{
		Size: float64(c.fontSize),
	})

	color := color.RGBA{uint8(rand.Intn(150)), uint8(rand.Intn(150)), uint8(rand.Intn(150)), 255}

	if c.useNoise {
		c.writeNoise(img)
	}
	if c.useCurve {
		c.writeCurve(img, color)
		c.writeCurve(img, color)
		c.writeCurve(img, color)
	}

	text := strings.Split(code.value, "")
	for index, char := range text {
		x := int(float64(c.fontSize)*float64(index+1)*1.5) + rand.Intn(10)
		y := c.fontSize + rand.Intn(20)
		angle := float64(rand.Intn(80)-40) * (3.1415926 / 180.0)

		c.drawText(img, face, color, x, y, angle, char)
	}

	var buf bytes.Buffer
	err = png.Encode(&buf, img)
	if err != nil {
		return "", "", err
	}

	return base64.StdEncoding.EncodeToString(buf.Bytes()), code.hash, nil
}

// 设置随机字体
func (c *Captcha) setRandomFont() error {
	files, err := filepath.Glob(filepath.Join(fontDir, "*.ttf"))
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return fmt.Errorf("no font files found in %s", fontDir)
	}
	fontPath := files[rand.Intn(len(files))]
	fontBytes, err := os.ReadFile(fontPath)
	if err != nil {
		return fmt.Errorf("error reading font file: %v", err)
	}
	font, err := truetype.Parse(fontBytes)
	if err != nil {
		return fmt.Errorf("error parsing font: %v [fontPath]=%s", err, fontPath)
	}
	fmt.Println("fontPath:%s\r\n", fontPath)
	c.font = font
	return nil
}

func (c *Captcha) drawCurveSegment(img *image.RGBA, color color.Color, startX, endX int, amplitude, phaseShift, frequency float64) {
	for px := startX; px <= endX; px++ {
		py := int(amplitude*math.Sin(frequency*float64(px)+phaseShift) + float64(c.imageH)/2)
		i := c.fontSize / 5
		for i > 0 {
			if px+i < c.imageW && py+i < c.imageH {
				img.Set(px+i, py+i, color)
			}
			i--
		}
	}
}

// 绘制曲线
func (c *Captcha) writeCurve(img *image.RGBA, color color.Color) {
	// 曲线前部分
	amplitude1 := float64(c.RandIntInRange(1, c.imageH/2))
	phaseShift1 := float64(c.RandIntInRange(-(c.imageH / 4), c.imageH/4))
	frequency1 := 2 * math.Pi / float64(c.RandIntInRange(c.imageH, c.imageW*2))
	startX1 := 0
	// endX1 := c.RandIntInRange(c.imageW/2, c.imageW)
	endX1 := c.imageW

	c.drawCurveSegment(img, color, startX1, endX1, amplitude1, phaseShift1, frequency1)

	// 曲线后部分
	// amplitude2 := float64(c.RandIntInRange(1, c.imageH/2))
	// phaseShift2 := float64(c.RandIntInRange(-(c.imageH / 4), c.imageH/4))
	// frequency2 := 2 * math.Pi / float64(c.RandIntInRange(c.imageH, c.imageW*2))
	// startX2 := endX1
	// endX2 := c.imageW

	// c.drawCurveSegment(img, color, startX2, endX2, amplitude2, phaseShift2, frequency2)
}

func (c *Captcha) writeNoise(img *image.RGBA) {
	codeSet := "1234567890abcdefhijkmnpqrstuvwxyz"
	for i := 0; i < 10; i++ {
		// 杂点颜色
		noiseColor := color.RGBA{uint8(c.RandIntInRange(150, 255)), uint8(c.RandIntInRange(150, 255)), uint8(c.RandIntInRange(150, 255)), 255}
		for j := 0; j < 5; j++ {
			// 绘杂点
			c.drawStringOnImage(img, string(codeSet[rand.Intn(len(codeSet))]), 18, c.RandIntInRange(-10, c.imageW), c.RandIntInRange(-10, c.imageH), noiseColor)
		}
	}

}

func (c *Captcha) RandIntInRange(min, max int) int {
	// 确保 max >= min，否则交换它们
	if max < min {
		min, max = max, min
	}
	// 计算范围大小
	rangeSize := max - min + 1
	// 使用 rand.Intn 生成一个 0 到 rangeSize-1 之间的随机整数
	randomInt := rand.Intn(rangeSize)
	// 将随机整数映射到 min 到 max 的范围内
	return randomInt + min
}

func (c *Captcha) drawStringOnImage(img *image.RGBA, text string, fontSize float64, x, y int, textColor color.Color) error {
	// 设置字体上下文
	ctx := freetype.NewContext()
	ctx.SetDPI(72)
	ctx.SetFont(c.font)
	ctx.SetFontSize(fontSize)
	ctx.SetClip(img.Bounds())
	ctx.SetDst(img)
	ctx.SetSrc(image.NewUniform(textColor))

	// 将字符串位置转换为 fixed.Point26_6 类型
	pt := fixed.P(x, y+int(ctx.PointToFixed(fontSize)>>6))

	// 绘制字符串
	_, err := ctx.DrawString(text, pt)
	if err != nil {
		return fmt.Errorf("error drawing string: %w", err)
	}

	return nil
}

func (c *Captcha) drawText(img *image.RGBA, face font.Face, color color.Color, x, y int, angle float64, text string) {
	d := &font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(color),
		Face: face,
	}
	d.Dot = freetype.Pt(x, y)
	d.DrawString(text)
}

func (c *Captcha) generateCode() struct {
	value string
	hash  string
} {
	characters := strings.Split(c.codeSet, "")
	bag := ""
	for i := 0; i < c.length; i++ {
		bag += characters[rand.Intn(len(characters))]
	}

	hash := fmt.Sprintf("%x", md5.Sum([]byte(strconv.FormatInt(time.Now().UnixNano(), 10)+randString(10)+bag)))
	conn, err := redis.Dial("tcp", ":6379")
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	conn.Do("SETEX", c.getRedisKey(hash), 300, bag)

	return struct {
		value string
		hash  string
	}{value: bag, hash: hash}
}

func (c *Captcha) getRedisKey(hash string) string {
	return "yunj.library.captcha:" + hash
}

func (c *Captcha) Check(code, hash string) bool {
	conn, err := redis.Dial("tcp", ":6379")
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	redisCode, err := redis.String(conn.Do("GET", c.getRedisKey(hash)))
	if err != nil {
		return false
	}

	res := strings.ToLower(redisCode) == strings.ToLower(code)
	if res {
		conn.Do("DEL", c.getRedisKey(hash))
	}
	return res
}

func randString(n int) string {
	const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}
	return string(b)
}

func msectime() int64 {
	return time.Now().UnixNano() / int64(time.Millisecond)
}

func sin(x float64) float64 {
	return math.Sin(x)
}
