package main

import (
  "time"
)

const (
  StateWatch = iota
  StateUnWatch
)

const MsgLink = 49

const (
  NoScript   = -1
  NoValue    = -2
  RangePrice = -3
)

type Task struct {
  ID         string     `json:"id,omitempty"`
  CreateTime time.Time  `json:"create_time,omitempty"`
  ReportTime time.Time  `json:"report_time,omitempty"`
  Payloads   []*Payload `json:"payloads,omitempty"`
}

type Payload struct {
  Message *Message `json:"message,omitempty"`
  Product *Product `json:"product,omitempty"`
}

type Message struct {
  ID      string `json:"id,omitempty"`
  URL     string `json:"url,omitempty"`
  Content string `json:"content,omitempty"`
}

type Product struct {
  AID        uint64    `json:"_id"`
  ID         string    `json:"id,omitempty"`
  URL        string    `json:"url,omitempty"`
  ShortURL   string    `json:"short_url,omitempty"`
  Source     int       `json:"source,omitempty"`
  Title      string    `json:"title,omitempty"`
  Currency   int       `json:"currency,omitempty"`
  Price      float64   `json:"price,omitempty"`
  PriceLow   float64   `json:"price_low,omitempty"`
  PriceHigh  float64   `json:"price_high,omitempty"`
  Stock      int       `json:"stock,omitempty"`
  Sales      int       `json:"sales,omitempty"`
  Category   string    `json:"category,omitempty"`
  Comments   Comments  `json:"comments,omitempty"`
  UpdateTime time.Time `json:"update_time,omitempty"`
}

func NewProduct() *Product {
  return &Product{
    Price: NoScript,
    Stock: NoScript,
    Sales: NoScript,
    Comments: Comments{
      Total: NoScript,
    },
  }
}

type Comments struct {
  Total  int `json:"total,omitempty"`
  Star5  int `json:"star5,omitempty"`
  Star4  int `json:"star4,omitempty"`
  Star3  int `json:"star3,omitempty"`
  Star2  int `json:"star2,omitempty"`
  Star1  int `json:"star1,omitempty"`
  Image  int `json:"image,omitempty"`
  Append int `json:"append,omitempty"`
}

type ProductWatch struct {
  UserID      string    `json:"user_id,omitempty"`
  ProductID   string    `json:"product_id,omitempty"`
  Currency    int       `json:"currency,omitempty"`
  Price       float64   `json:"price,omitempty"`
  PriceLow    float64   `json:"price_low,omitempty"`
  PriceHigh   float64   `json:"price_high,omitempty"`
  Stock       int       `json:"stock,omitempty"`
  WatchTime   time.Time `json:"watch_time,omitempty"`
  UnWatchTime time.Time `json:"unwatch_time,omitempty"`
  // 0：关注，1：取消关注
  State int `json:"state,omitempty"`
  // 0：不提醒，1：按价格，2：按比例
  Rdo int     `json:"remind_decrease_option,omitempty"`
  Rdv float64 `json:"remind_decrease_value,omitempty"`
  Rio int     `json:"remind_increase_option,omitempty"`
  Riv float64 `json:"remind_increase_value,omitempty"`
}
