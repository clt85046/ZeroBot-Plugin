package score

import (
	"os"
	"github.com/jinzhu/gorm"
)

// sdb 标记数据库
var mdb *bjdb

// scoredb 标记数据库
type bjdb gorm.DB

// scoretable 标记结构体
type bjtable struct {
	UID   int64 `gorm:"column:uid;primary_key"`
	Score int   `gorm:"column:score;default:0"`
}

// TableName ...
func (bjtable) TableName() string {
	return "bj"
}


// initialize 初始化ScoreDB数据库
func initializeBJ(dbpath string) *bjdb {
	var err error
	if _, err = os.Stat(dbpath); err != nil || os.IsNotExist(err) {
		// 生成文件
		f, err := os.Create(dbpath)
		if err != nil {
			return nil
		}
		defer f.Close()
	}
	gdb, err := gorm.Open("sqlite3", dbpath)
	if err != nil {
		panic(err)
	}
	gdb.AutoMigrate(&bjtable{})
	return (*bjdb)(gdb)
}

// Close ...
func (mdb *bjdb) CloseBJ() error {
	db := (*gorm.DB)(mdb)
	return db.Close()
}

// GetScoreByUID 取得分数
func (mdb *bjdb) GetBJScoreByUID(uid int64) (s bjtable) {
	db := (*gorm.DB)(mdb)
	db.Model(&bjtable{}).FirstOrCreate(&s, "uid = ? ", uid)
	return s
}

// InsertOrUpdateScoreByUID 插入或更新分数
func (mdb *bjdb) InsertOrUpdateBJScoreByUID(uid int64, score int) (err error) {
	db := (*gorm.DB)(mdb)
	s := bjtable{
		UID:   uid,
		Score: score,
	}
	if err = db.Model(&bjtable{}).First(&s, "uid = ? ", uid).Error; err != nil {
		// error handling...
		if gorm.IsRecordNotFoundError(err) {
			err = db.Model(&bjtable{}).Create(&s).Error // newUser not user
		}
	} else {
		err = db.Model(&bjtable{}).Where("uid = ? ", uid).Update(
			map[string]any{
				"score": score,
			}).Error
	}
	return
}



func (mdb *bjdb) GetBJScoreRankByTopN(n int) (st []bjtable, err error) {
	db := (*gorm.DB)(mdb)
	err = db.Model(&bjtable{}).Order("score desc").Limit(n).Find(&st).Error
	return
}
