package main

import (
  "encoding/base64"
  "encoding/json"
  "fmt"
  "math"
  "time"

  "github.com/kwf2030/commons/beanstalk"
  "github.com/kwf2030/commons/times"
)

func reserveJob(conn *beanstalk.Conn) (string, *Task) {
  _, e := conn.Watch(Conf.Beanstalk.ReserveTube)
  if e != nil {
    logger.Error().Err(e).Msg("ERR: Watch")
    return "", nil
  }
  e = conn.Use(Conf.Beanstalk.ReserveTube)
  if e != nil {
    logger.Error().Err(e).Msg("ERR: Use")
    return "", nil
  }
  _, e = conn.Ignore("default")
  if e != nil {
    logger.Error().Err(e).Msg("ERR: Ignore")
    return "", nil
  }
  id, job, e := conn.ReserveWithTimeout(Conf.Beanstalk.ReserveTimeout)
  if e != nil {
    if e != beanstalk.ErrTimedOut {
      logger.Error().Err(e).Msg("ERR: ReserveWithTimeout")
    }
    return "", nil
  }
  data := make([]byte, base64.RawStdEncoding.DecodedLen(len(job)))
  _, e = base64.RawStdEncoding.Decode(data, job)
  if e != nil {
    logger.Error().Err(e).Msg("ERR: Decode")
    return "", nil
  }
  t := &Task{}
  e = json.Unmarshal(data, t)
  if e != nil {
    logger.Error().Err(e).Msg("ERR: Unmarshal")
    return "", nil
  }
  dump(fmt.Sprintf("%s/dump/%s_pt.json", Conf.Log.Dir, t.ID), data)
  logger.Info().Msgf("reserve job, ok, job id=%s, %d items", id, len(t.Payloads))
  return id, t
}

func collectChanged(t *Task) []string {
  ret := make([]string, 0, len(t.Payloads))
  tx, _ := db.Begin()
  defer tx.Commit()
  for _, payload := range t.Payloads {
    msg := payload.Message
    p := payload.Product
    if p == nil || p.ID == "" || p.Price == NoScript || p.Price == NoValue {
      continue
    }
    aid := 0
    // 新增product_watch记录（不存在时）
    // 更新product_watch的watch_time和state字段（存在且state为1时）
    if msg != nil && msg.ID != "" {
      var uid string
      var ct time.Time
      tx.QueryRow(`SELECT from_user_id, create_time FROM msg WHERE id=? LIMIT 1`, msg.ID).Scan(&uid, &ct)
      if uid != "" {
        wt := ct.Format(times.DateTimeSFormat)
        var state int
        tx.QueryRow(`SELECT _id, state FROM product_watch WHERE user_id=? AND product_id=? LIMIT 1`, uid, p.ID).Scan(&aid, &state)
        if aid == 0 {
          tx.Exec(`INSERT INTO product_watch (user_id, product_id, currency, price, price_low, price_high, stock, watch_time, state) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
            uid, p.ID, p.Currency, p.Price,
            p.PriceLow, p.PriceHigh, p.Stock, wt, StateWatch)
        } else if state == StateUnWatch {
          tx.Exec(`UPDATE product_watch SET watch_time=?, state=? WHERE user_id=? AND product_id=?`, wt, StateWatch, uid, p.ID)
        }
      }
    }

    ut := p.UpdateTime.Format(times.DateTimeSFormat)
    var price, priceLow, priceHigh float64 = NoValue, 0, 0
    var stock int
    tx.QueryRow(`SELECT price, price_low, price_high, stock FROM product_update WHERE id=? ORDER BY update_time DESC LIMIT 1`, p.ID).Scan(&price, &priceLow, &priceHigh, &stock)
    if validateChanged(p, price, priceLow, priceHigh) {
      // 记录下有变动的productID
      ret = append(ret, p.ID)
      // 新增price_update记录
      var comments string
      if p.Comments.Total > 0 {
        data, _ := json.Marshal(p.Comments)
        comments = string(data)
      }
      tx.Exec(`INSERT INTO product_update (id, source, url, short_url, title, currency, price, price_low, price_high, stock, sales, category, comments, update_time) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
        p.ID, p.Source, p.URL, p.ShortURL, p.Title,
        p.Currency, p.Price, p.PriceLow, p.PriceHigh, p.Stock,
        p.Sales, p.Category, comments, ut)

      // 新增product记录（不存在时）
      // 更新product记录（已存在时）
      aid = 0
      tx.QueryRow(`SELECT _id FROM product WHERE id=? LIMIT 1`, p.ID).Scan(&aid)
      if aid == 0 {
        // 新增记录时注意要加上last_dispatch_time字段
        tx.Exec(`INSERT INTO product (id, source, url, short_url, title, currency, price, price_low, price_high, stock, sales, category, comments, update_time, last_dispatch_time) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
          p.ID, p.Source, p.URL, p.ShortURL, p.Title,
          p.Currency, p.Price, p.PriceLow, p.PriceHigh, p.Stock,
          p.Sales, p.Category, comments, ut, ut)
      } else {
        // 不需要更新last_dispatch_time字段，因为之前分发任务的时候已经更新过了
        tx.Exec(`UPDATE product SET source=?, url=?, short_url=?, title=?, currency=?, price=?, price_low=?, price_high=?, stock=?, sales=?, category=?, comments=?, update_time=? WHERE id=?`,
          p.Source, p.URL, p.ShortURL, p.Title, p.Currency,
          p.Price, p.PriceLow, p.PriceHigh, p.Stock, p.Sales,
          p.Category, comments, ut, p.ID)
      }
    }
  }
  logger.Info().Msgf("collect changed, ok, %d items changed", len(ret))
  return ret
}

// price/price_low/price_high任一字段变动
func validateChanged(p *Product, price, priceLow, priceHigh float64) bool {
  if price == RangePrice && p.Price == RangePrice {
    if priceLow < 0 || priceHigh < 0 || p.PriceLow < 0 || p.PriceHigh < 0 {
      return false
    }
    return priceLow != p.PriceLow || priceHigh != p.PriceHigh
  }
  if price >= 0 && p.Price >= 0 {
    return price != p.Price
  }
  return false
}

func putMsgJob(conn *beanstalk.Conn, products []string) {
  e := conn.Use(Conf.Beanstalk.PutTubeMsg)
  if e != nil {
    logger.Error().Err(e).Msg("ERR: Use")
    return
  }
  m := createPushMsg(products)
  if len(m) <= 0 {
    logger.Info().Msg("no msg to push")
    return
  }
  // 推送消息分两种，
  // 一种是by_user：用户-->消息列表，按用户推送消息，
  // 一种是by_text：消息-->用户列表，按消息推送用户，
  // {"by_user": [{"user1": ["text1", "text2"]}, {"user2": ["text3", "text4"]}], "by_text": [{"text1": ["user1", "user2"]}, {"text2": ["user3", "user4"]}]}
  ct := times.NowStrFormat(times.DateTimeFormat3)
  data, _ := json.Marshal(map[string]interface{}{"by_user": m, "create_time": ct})
  _, e = conn.Put(Conf.Beanstalk.PutTubePriority, Conf.Beanstalk.PutTubeDelay, Conf.Beanstalk.PutTubeTTR, data)
  if e != nil {
    logger.Error().Err(e).Msg("ERR: Put")
    return
  }
  dump(fmt.Sprintf("%s/dump/%s_m.json", Conf.Log.Dir, ct), data)
  logger.Info().Msg("put msg job, ok")
}

func createPushMsg(products []string) map[string][]string {
  ret := make(map[string][]string, len(products)*10)
  tx, _ := db.Begin()
  defer tx.Rollback()
  for _, v := range products {
    if v == "" {
      continue
    }
    p := NewProduct()
    var comments string
    r := tx.QueryRow(`SELECT _id, id, source, url, short_url, title, currency, price, price_low, price_high, stock, sales, category, comments, update_time FROM product WHERE id=? LIMIT 1`, v)
    e := r.Scan(&p.AID, &p.ID, &p.Source, &p.URL, &p.ShortURL,
      &p.Title, &p.Currency, &p.Price, &p.PriceLow, &p.PriceHigh,
      &p.Stock, &p.Sales, &p.Category, &comments, &p.UpdateTime)
    if e != nil {
      logger.Error().Err(e).Msg("ERR: Scan")
      continue
    }
    if p.ID == "" || p.Price == NoScript || p.Price == NoValue {
      continue
    }
    rows, e := tx.Query(`SELECT user_id, currency, price, price_low, price_high, stock, watch_time, remind_decrease_option, remind_decrease_value, remind_increase_option, remind_increase_value FROM product_watch WHERE product_id=? AND state=0`, v)
    if e != nil {
      logger.Error().Err(e).Msg("ERR: Query")
      continue
    }
    for rows.Next() {
      pw := &ProductWatch{ProductID: v}
      e := rows.Scan(&pw.UserID, &pw.Currency, &pw.Price, &pw.PriceLow, &pw.PriceHigh,
        &pw.Stock, &pw.WatchTime, &pw.Rdo, &pw.Rdv, &pw.Rio, &pw.Riv)
      if e != nil {
        logger.Error().Err(e).Msg("ERR: Scan")
        continue
      }
      if pw.UserID == "" || pw.Price == NoScript || pw.Price == NoValue {
        continue
      }
      // 0：不提醒，1：按价格，2：按比例
      if pw.Rdo == 0 && pw.Rio == 0 {
        continue
      }
      msg := concatMsg(p, pw)
      if msg == "" {
        continue
      }
      if _, ok := ret[pw.UserID]; !ok {
        ret[pw.UserID] = make([]string, 0, 2)
      }
      ret[pw.UserID] = append(ret[pw.UserID], msg)
    }
  }
  return ret
}

func concatMsg(p *Product, pw *ProductWatch) string {
  p1 := p.Price
  p2 := pw.Price
  if p1 >= 0 && p2 >= 0 {
    switch {
    case p1 == p2:
      return ""

    case p1 < p2:
      // 降价
      if pw.Rdo == 0 {
        return ""
      }
      rg := (1 - p1/p2) * 100
      logger.Debug().Msgf("%s[%.2f, %.2f], %.2f%%", p.ID, p2, p1, rg)
      if pw.Rdo == 1 {
        if pw.Rdv < p.Price {
          return ""
        }
      } else if pw.Rdo == 2 {
        if pw.Rdv > rg {
          return ""
        }
      }
      r := []rune(p.Title)
      if len(r) > 30 {
        r = r[:30]
        p.Title = string(r) + "..."
      }
      return fmt.Sprintf("%s 降价了，关注价%s 现价%s 降幅%d%% %s", p.Title, fmt.Sprintf(getCurrencyFormat(pw.Currency), p2), fmt.Sprintf(getCurrencyFormat(p.Currency), p1), int(math.Round(rg)), p.ShortURL)

    case p1 > p2:
      // 涨价
      if pw.Rio == 0 {
        return ""
      }
      rg := (p1/p2 - 1) * 100
      logger.Debug().Msgf("%s[%.2f, %.2f], %.2f%%", p.ID, p2, p1, rg)
      if pw.Rio == 1 {
        if pw.Riv > p.Price {
          return ""
        }
      } else if pw.Rio == 2 {
        if pw.Riv > rg {
          return ""
        }
      }
      r := []rune(p.Title)
      if len(r) > 30 {
        r = r[:30]
        p.Title = string(r) + "..."
      }
      return fmt.Sprintf("%s 涨价了，关注价%s 现价%s 涨幅%d%% %s", p.Title, fmt.Sprintf(getCurrencyFormat(pw.Currency), p2), fmt.Sprintf(getCurrencyFormat(p.Currency), p1), int(math.Round(rg)), p.ShortURL)
    }
  }
  if p1 == RangePrice && p2 == RangePrice {
    if math.Abs(p.PriceLow-pw.PriceLow) >= 1 || math.Abs(p.PriceHigh-pw.PriceHigh) >= 1 {
      return fmt.Sprintf("%s 价格有变动，关注价[%s-%s] 现价[%s-%s] %s", p.Title, fmt.Sprintf(getCurrencyFormat(pw.Currency), pw.PriceLow), fmt.Sprintf(getCurrencyFormat(pw.Currency), pw.PriceHigh), fmt.Sprintf(getCurrencyFormat(p.Currency), p.PriceLow), fmt.Sprintf(getCurrencyFormat(p.Currency), p.PriceHigh), p.ShortURL)
    }
  }
  return ""
}

func getCurrencyFormat(currency int) string {
  // 0:RMB, 1:JPY, 2:USD, 3:GBP, 4:EUR
  switch currency {
  case 0:
    return "￥%.2f"
  case 1:
    return "¥%.2f"
  case 2:
    return "$%.2f"
  case 3:
    return "£%.2f"
  case 4:
    return "€%.2f"
  }
  return "￥%.2f"
}
