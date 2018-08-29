package main

import (
  "io/ioutil"

  "gopkg.in/yaml.v2"
)

var Conf = &struct {
  Log       LogConf       `yaml:"log"`
  Beanstalk BeanstalkConf `yaml:"beanstalk"`
  Database  DatabaseConf  `yaml:"database"`
  Task      TaskConf      `yaml:"task"`
}{}

type LogConf struct {
  Dir   string `yaml:"dir"`
  Level string `yaml:"level"`
}

type BeanstalkConf struct {
  Host            string `yaml:"host"`
  Port            int    `yaml:"port"`
  ReserveTube     string `yaml:"reserve_tube"`
  ReserveTimeout  int    `yaml:"reserve_timeout"`
  PutTubeTask     string `yaml:"put_tube_task"`
  PutTubeMsg      string `yaml:"put_tube_msg"`
  PutTubePriority int    `yaml:"put_tube_priority"`
  PutTubeDelay    int    `yaml:"put_tube_delay"`
  PutTubeTTR      int    `yaml:"put_tube_ttr"`
}

type DatabaseConf struct {
  Host     string            `yaml:"host"`
  Port     int               `yaml:"port"`
  DB       string            `yaml:"db"`
  User     string            `yaml:"user"`
  Password string            `yaml:"password"`
  Params   map[string]string `yaml:"params"`
}

type TaskConf struct {
  PollingInterval  int `yaml:"polling_interval"`
  DispatchDuration int `yaml:"dispatch_duration"`
  Overload         int `yaml:"overload"`
}

func LoadConf(file string) error {
  data, e := ioutil.ReadFile(file)
  if e != nil {
    return e
  }
  return yaml.Unmarshal(data, Conf)
}
