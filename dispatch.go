package main

import (
  "database/sql"
  "encoding/json"
  "fmt"
  "strconv"
  "time"

  "github.com/kwf2030/commons/times"
  "github.com/rs/xid"
)

func putRunnerJob() {
  arr1 := checkMsg(Conf.Task.Overload)
  arr2 := checkProduct(Conf.Task.Overload - len(arr1))
  payloads := make([]*Payload, 0, Conf.Task.Overload)
  for _, v := range arr1 {
    payloads = append(payloads, v)
  }
  for _, v := range arr2 {
    payloads = append(payloads, v)
  }
  if len(payloads) == 0 {
    logger.Debug().Msg("no data to dispatch")
    return
  }
  now := times.Now()
  tid := xid.New().String()
  t := &Task{
    ID:         tid,
    CreateTime: now,
    Payloads:   payloads,
  }
  saveDispatchTime(arr2, now)
  e := conn.Use(Conf.Beanstalk.PutTubeTask)
  if e != nil {
    logger.Error().Err(e).Msg("ERR: Use")
    return
  }
  data, _ := json.Marshal(t)
  dump(fmt.Sprintf("%s/dump/%s_runner.json", Conf.Log.Dir, tid), data)
  _, e = conn.Put(Conf.Beanstalk.PutTubePriority, Conf.Beanstalk.PutTubeDelay, Conf.Beanstalk.PutTubeTTR, data)
  if e != nil {
    logger.Error().Err(e).Msg("ERR: Put")
    return
  }
  logger.Info().Msgf("put runner job, ok, dispatch %d items, task id=%s", len(payloads), tid)
}

func checkMsg(limit int) []*Payload {
  if limit <= 0 {
    return nil
  }
  ret := make([]*Payload, 0, limit)
  rows, e := db.Query(`SELECT _id, id, type, content, url FROM msg WHERE _id>? AND (type=1 OR type=49) LIMIT ?`, lastCheckMsg, limit)
  if e != nil {
    return nil
  }
  defer rows.Close()
  var aid uint64
  var msgType int
  for rows.Next() {
    msg := &Message{}
    e := rows.Scan(&aid, &msg.ID, &msgType, &msg.Content, &msg.URL)
    if e != nil || (msg.Content == "" && msg.URL == "") {
      continue
    }
    // 如果是分享且URL有值的话，删掉Content减少传输量
    if msgType == MsgLink && msg.URL != "" {
      msg.Content = ""
    }
    ret = append(ret, &Payload{Message: msg})
  }
  if aid > 0 {
    saveLastCheckMsg(aid)
  }
  logger.Debug().Msg("check msg, ok")
  return ret
}

func checkProduct(limit int) []*Payload {
  if limit <= 0 {
    return nil
  }
  tx, _ := db.Begin()
  defer tx.Rollback()
  ret := make([]*Payload, 0, limit)
  var stmt, dt string
  var rows, rows2 *sql.Rows
  var e error
  if Conf.Task.DispatchDuration <= 0 {
    stmt = `SELECT _id, id, url FROM product WHERE _id>? LIMIT ?`
    rows, e = tx.Query(stmt, lastCheckProduct, limit)
  } else {
    dt = times.Now().Add(time.Minute * time.Duration(-Conf.Task.DispatchDuration)).Format(times.DateTimeSFormat)
    stmt = `SELECT _id, id, url FROM product WHERE _id>? AND last_dispatch_time<? LIMIT ?`
    rows, e = tx.Query(stmt, lastCheckProduct, dt, limit)
  }
  if e != nil {
    return nil
  }
  defer rows.Close()
  var aid uint64
  for rows.Next() {
    p := &Product{}
    e := rows.Scan(&p.AID, &p.ID, &p.URL)
    if e != nil || p.URL == "" {
      continue
    }
    aid = p.AID
    ret = append(ret, &Payload{Product: p})
  }
  if aid > 0 {
    saveLastCheckProduct(aid)
  }
  l := len(ret)
  if l >= limit {
    return ret
  }

  n := countProduct() - l
  lm := limit - l
  if lm > n {
    lm = n
  }
  if Conf.Task.DispatchDuration <= 0 {
    rows2, e = tx.Query(stmt, 0, lm)
  } else {
    rows2, e = tx.Query(stmt, 0, dt, lm)
  }
  if e != nil {
    return ret
  }
  defer rows2.Close()
  aid = 0
  for rows2.Next() {
    p := &Product{}
    e := rows2.Scan(&p.AID, &p.ID, &p.URL)
    if e != nil || p.URL == "" {
      continue
    }
    aid = p.AID
    ret = append(ret, &Payload{Product: p})
  }
  if aid > 0 {
    saveLastCheckProduct(aid)
  }
  logger.Debug().Msg("check product, ok")
  return ret
}

func countProduct() int {
  ret := -1
  db.QueryRow(`SELECT COUNT(_id) FROM product`).Scan(&ret)
  return ret
}

func saveLastCheckMsg(aid uint64) {
  lastCheckMsg = aid
  kv.UpdateV(bucketVar, lastCheckMsgKey, []byte(strconv.FormatUint(aid, 10)))
}

func saveLastCheckProduct(aid uint64) {
  lastCheckProduct = aid
  kv.UpdateV(bucketVar, lastCheckProductKey, []byte(strconv.FormatUint(aid, 10)))
}

func saveDispatchTime(arr []*Payload, t time.Time) {
  tx, _ := db.Begin()
  defer tx.Commit()
  str := t.Format(times.DateTimeSFormat)
  for _, v := range arr {
    tx.Exec(`UPDATE product SET last_dispatch_time=? WHERE _id=?`, str, v.Product.AID)
  }
}
