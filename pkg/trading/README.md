# 日线股票交易系统

一个实时监控多只股票并根据预设策略生成买卖信号的日线交易系统。

## 功能特点

### 核心功能
- ✅ **实时股票监控**: 持续监控多只股票的实时数据
- ✅ **策略化交易**: 基于移动平均线(MA)交叉策略生成交易信号
- ✅ **智能信号生成**: 自动生成买入/卖出信号,包含:
  - 信号类型 (买入/卖出/持有)
  - 建议价格
  - 执行时间建议
  - 置信度水平
  - 详细理由说明
- ✅ **持仓管理**: 跟踪当前持仓和盈亏情况
- ✅ **历史数据存储**: 存储股票数据和交易信号供后续分析

## 系统架构

```
pkg/trading/
├── types.go              # 核心数据类型定义
├── provider/             # 数据提供者
│   ├── interface.go      # 提供者接口
│   └── mock.go          # 模拟提供者(用于测试)
├── strategy/            # 交易策略
│   ├── interface.go     # 策略接口
│   └── ma_cross.go      # MA交叉策略实现
├── signal/              # 信号生成
│   └── generator.go     # 信号生成器
├── storage/             # 数据存储
│   └── memory.go        # 内存存储实现
└── engine/              # 交易引擎
    └── engine.go        # 主引擎逻辑
```

## 快速开始

### 1. 构建系统

```bash
# 构建可执行文件
go build -o bin/trading cmd/trading/main.go

# 运行
./bin/trading
```

### 2. 配置系统

编辑 `cmd/trading/main.go` 进行配置:

```go
// 要监控的股票代码
symbols := []string{"AAPL", "GOOGL", "MSFT", "TSLA", "AMZN"}

// 数据更新间隔
updateInterval := 10 * time.Second

// 交易策略配置
strategies := []engine.Strategy{
    strategy.NewMACrossStrategy(5, 20),   // 短期策略: 5日/20日均线
    strategy.NewMACrossStrategy(10, 50),  // 中期策略: 10日/50日均线
}

// 设置初始持仓(可选)
eng.UpdatePosition("AAPL", 100, 150.0)  // 100股 @ $150
```

### 3. 运行示例

```bash
=================================================
       Daily Stock Trading System
=================================================

Monitoring stocks: [AAPL GOOGL MSFT TSLA AMZN]
Update interval: 10s

Active strategies:
  1. MA_Cross_5_20
  2. MA_Cross_10_50

Trading engine started, monitoring 5 symbols: [AAPL GOOGL MSFT TSLA AMZN]
System running... Press Ctrl+C to stop
```

## 交易策略说明

### MA交叉策略 (Moving Average Crossover)

系统使用移动平均线交叉策略生成交易信号:

#### 1. 金叉 (Golden Cross) - 买入信号
- **触发条件**: 短期均线上穿长期均线
- **执行条件**: 当前未持有该股票
- **含义**: 趋势转为上涨,建议买入

#### 2. 死叉 (Death Cross) - 卖出信号
- **触发条件**: 短期均线下穿长期均线
- **执行条件**: 当前持有该股票
- **含义**: 趋势转为下跌,建议卖出

**策略参数**:
- `shortPeriod`: 短期均线周期 (例如: 5天, 10天)
- `longPeriod`: 长期均线周期 (例如: 20天, 50天)

## 交易信号示例

当系统检测到交易机会时,会生成如下格式的信号:

```
================================================================================
🔔 NEW TRADING SIGNAL
================================================================================
Symbol:      AAPL
Action:      BUY
Price:       $175.50
Confidence:  85.0%
Generated:   2026-02-04 10:30:00
Execute At:  2026-02-05 09:30:00
Reason:      Golden cross: MA5(176.20) > MA20(174.80)
================================================================================
⏰ Execute in: 23.0 hours
📅 Suggested execution time: 2026-02-05 09:30:00 (Wednesday)
```

## 信号详细说明

每个交易信号包含以下信息:

| 字段 | 说明 |
|------|------|
| **Symbol** | 股票代码 |
| **Action** | 操作类型: BUY(买入), SELL(卖出), HOLD(持有) |
| **Price** | 信号生成时的当前价格 |
| **Confidence** | 信号置信度 (0-100%) |
| **Generated** | 信号生成时间 |
| **Execute At** | 建议执行时间(下一个交易时段开盘时) |
| **Reason** | 生成信号的详细原因 |

### 执行时间说明

- **系统只生成信号,不执行实际交易**
- 建议执行时间为下一个交易日的开盘时间(9:30 AM)
- 自动跳过周末,计算下一个工作日
- 系统会显示距离建议执行时间的倒计时

## 扩展开发

### 添加新的数据提供者

实现 `provider.DataProvider` 接口:

```go
type DataProvider interface {
    GetStockData(ctx context.Context, symbols []string) ([]*trading.StockData, error)
    Subscribe(ctx context.Context, symbols []string, callback func(*trading.StockData)) error
    Close() error
}
```

### 添加新的交易策略

实现 `engine.Strategy` 接口:

```go
type Strategy interface {
    Name() string
    Analyze(data []*trading.StockData, positions map[string]*trading.Position) ([]*trading.TradingSignal, error)
}
```

### 策略开发示例

可以实现以下策略:

1. **RSI策略**: 基于相对强弱指标
2. **MACD策略**: 使用MACD指标交叉
3. **布林带策略**: 基于波动率通道
4. **成交量分析**: 结合价格和成交量
5. **多因子策略**: 综合多个技术指标

## 项目结构

```
agentic-coder/
├── cmd/trading/          # 主程序入口
│   └── main.go
├── pkg/trading/          # 核心交易逻辑
│   ├── types.go         # 数据类型定义
│   ├── provider/        # 数据提供者
│   ├── strategy/        # 交易策略
│   ├── signal/          # 信号生成器
│   ├── storage/         # 数据存储
│   └── engine/          # 交易引擎
├── examples/trading/     # 示例和文档
│   ├── README.md
│   └── config.example.yaml
└── bin/                 # 编译输出
    └── trading
```

## 重要提示

⚠️ **风险警告**:
- 本系统仅生成交易信号,不执行实际交易
- 当前使用模拟数据提供者进行演示
- 生产环境需接入真实市场数据API
- 交易有风险,投资需谨慎
- 请在实际交易前进行充分的回测和验证

## 生产环境部署

### 接入真实数据源

可以集成以下数据提供商:

- **Alpha Vantage**: 免费/付费API
- **Yahoo Finance**: yfinance库
- **IEX Cloud**: 实时市场数据
- **Polygon.io**: 专业级数据服务
- **Tushare**: 中国A股数据

### 风险管理建议

1. **资金管理**: 设置单笔交易最大金额
2. **止损止盈**: 添加自动止损和止盈逻辑
3. **仓位控制**: 限制单只股票持仓比例
4. **分散投资**: 不要集中投资单一股票
5. **回测验证**: 使用历史数据进行策略回测

## 技术栈

- **语言**: Go 1.22+
- **依赖**: 无外部依赖(核心系统)
- **架构**: 模块化、可扩展设计

## 性能特点

- 支持同时监控多只股票
- 并发处理多个策略
- 低延迟信号生成
- 内存高效存储

## 后续开发计划

- [ ] 支持更多技术指标
- [ ] 添加回测功能
- [ ] Web界面展示
- [ ] 实时图表可视化
- [ ] 数据库持久化
- [ ] REST API接口
- [ ] 风险管理模块
- [ ] 邮件/短信通知

## License

Part of the agentic-coder project.
