package main

import (
  "database/sql"
  "fmt"
  "io/ioutil"
  "os"
  "os/signal"
  "strconv"
  "strings"
  "time"

  "github.com/kwf2030/commons/beanstalk"
  "github.com/kwf2030/commons/boltdb"
  "github.com/kwf2030/commons/times"
  "github.com/rs/zerolog"
  "go.etcd.io/bbolt"
  "github.com/go-sql-driver/mysql"
  "errors"
)

const Version = "0.1.0"

var (
  bucketVar = []byte("var")

  loopChan = make(chan struct{})

  logFile *os.File
  logger  *zerolog.Logger

  db *sql.DB
  kv *boltdb.KVStore

  lastCheckMsgKey = []byte("last_check_msg")
  lastCheckMsg    uint64

  lastCheckProductKey = []byte("last_check_product")
  lastCheckProduct    uint64

  conn *beanstalk.Conn
)

func main() {
  file := "conf.yaml"
  if len(os.Args) == 2 {
    file = os.Args[1]
  }
  e := LoadConf(file)
  if e != nil {
    panic(e)
  }

  initLogger()
  defer logFile.Close()
  logger.Info().Msg("Hiprice Dispatcher " + Version)

  initDB()
  defer db.Close()

  initKV()
  defer kv.Close()

  loadVars()

  initBeanstalk()
  defer conn.Quit()

  go run()
  loopChan <- struct{}{}

  s := make(chan os.Signal, 1)
  signal.Notify(s, os.Interrupt)
  <-s
}

func initLogger() {
  dir := Conf.Log.Dir
  e := os.MkdirAll(dir+"/dump", os.ModePerm)
  if e != nil {
    panic(e)
  }
  l := zerolog.DebugLevel
  switch strings.ToLower(Conf.Log.Level) {
  case "info":
    l = zerolog.InfoLevel
  case "warn":
    l = zerolog.WarnLevel
  case "error":
    l = zerolog.ErrorLevel
  case "fatal":
    l = zerolog.FatalLevel
  case "disable":
    l = zerolog.Disabled
  }
  zerolog.SetGlobalLevel(l)
  zerolog.TimeFieldFormat = ""
  if logFile != nil {
    logFile.Close()
  }
  logFile, _ = os.Create(fmt.Sprintf("%s/dispatcher_%s.log", dir, times.NowStrFormat(times.DateFormat3)))
  lg := zerolog.New(logFile).Level(l).With().Timestamp().Logger()
  logger = &lg
  now := times.Now()
  next := now.Add(time.Hour * 24)
  next = time.Date(next.Year(), next.Month(), next.Day(), 0, 0, 0, 0, next.Location())
  time.AfterFunc(next.Sub(now), func() {
    logger.Info().Msg("create log file")
    go initLogger()
  })
}

func initDB() {
  for i := 0; i < 3; i++ {
    c := mysql.NewConfig()
    c.Net = "tcp"
    c.Addr = fmt.Sprintf("%s:%d", Conf.Database.Host, Conf.Database.Port)
    c.Collation = "utf8mb4_unicode_ci"
    c.User = Conf.Database.User
    c.Passwd = Conf.Database.Password
    c.DBName = Conf.Database.DB
    c.Loc = times.TimeZoneSH
    c.ParseTime = true
    c.Params = Conf.Database.Params
    var e error
    db, e = sql.Open("mysql", c.FormatDSN())
    if e != nil {
      logger.Error().Err(e).Msg("database connect failed, will retry 30 seconds later")
      time.Sleep(time.Second * 30)
      continue
    }
    e = db.Ping()
    if e != nil {
      logger.Error().Err(e).Msg("database ping failed, will retry 10 seconds later")
      time.Sleep(time.Second * 10)
      continue
    }
    break
  }
  if db == nil {
    panic(errors.New("no database connection"))
  }
}

func initKV() {
  var e error
  kv, e = boltdb.Open("dispatcher.db", "var")
  if e != nil {
    panic(e)
  }
}

func loadVars() {
  kv.QueryB(bucketVar, func(b *bbolt.Bucket) error {
    v1 := b.Get(lastCheckMsgKey)
    if len(v1) > 0 {
      lastCheckMsg, _ = strconv.ParseUint(string(v1), 10, 64)
    }
    v2 := b.Get(lastCheckProductKey)
    if len(v2) > 0 {
      lastCheckProduct, _ = strconv.ParseUint(string(v2), 10, 64)
    }
    return nil
  })
  logger.Info().Msgf("last_check_msg=%d, last_check_product=%d", lastCheckMsg, lastCheckProduct)
}

func initBeanstalk() {
  for i := 0; i < 3; i++ {
    var e error
    conn, e = beanstalk.Dial(Conf.Beanstalk.Host, Conf.Beanstalk.Port)
    if e != nil {
      logger.Info().Msg("beanstalk connect failed, will retry 30 seconds later")
      time.Sleep(time.Second * 30)
      continue
    }
    _, e = conn.Watch(Conf.Beanstalk.ReserveTube)
    if e != nil {
      panic(e)
    }
    break
  }
}

func run() {
  // 外层循环是定时任务
  for range loopChan {
    // 内层循环是一直取任务直到没有为止
    for {
      id, task := reserveJob()
      if id == "" || task == nil || len(task.Payloads) == 0 {
        break
      }
      // 获取所有的价格较上次更新有变动的商品ID
      arr := collectChanged(task)
      if len(arr) > 0 {
        putMsgJob(arr)
      }
      e := conn.Delete(id)
      if e != nil {
        logger.Error().Err(e).Msg("ERR: Delete")
      }
    }
    putRunnerJob()
    scheduleNextTime()
  }
}

func scheduleNextTime() {
  logger.Info().Msg("schedule next time")
  time.AfterFunc(time.Minute*time.Duration(Conf.Task.PollingInterval), func() {
    loopChan <- struct{}{}
  })
}

func dump(file string, data []byte) {
  if file == "" || len(data) == 0 {
    return
  }
  ioutil.WriteFile(file, data, os.ModePerm)
}
