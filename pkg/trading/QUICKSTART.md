# 快速开始指南

## 5分钟上手日线股票交易系统

### 步骤1: 构建系统

```bash
cd /path/to/agentic-coder
go build -o bin/trading cmd/trading/main.go
```

### 步骤2: 运行系统

```bash
./bin/trading
```

你会看到类似输出:

```
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

### 步骤3: 等待交易信号

系统会每10秒更新一次数据。当检测到交易机会时,会显示信号:

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

### 步骤4: 根据信号操作

**重要**: 系统只生成信号,不会自动交易!

根据信号信息:
1. **Symbol**: 要交易的股票
2. **Action**: BUY(买入) 或 SELL(卖出)
3. **Price**: 参考价格
4. **Execute At**: 建议在这个时间执行交易
5. **Confidence**: 信号可靠性(越高越好)
6. **Reason**: 为什么生成这个信号

### 步骤5: 停止系统

按 `Ctrl+C` 停止系统,会显示最近的信号摘要。

## 自定义配置

### 修改监控的股票

编辑 `cmd/trading/main.go`:

```go
symbols := []string{"AAPL", "GOOGL", "MSFT", "TSLA", "AMZN"}
```

改为你想监控的股票代码。

### 修改更新频率

```go
updateInterval := 10 * time.Second  // 10秒更新一次
```

可以改为:
- `5 * time.Second` - 5秒(更频繁)
- `30 * time.Second` - 30秒
- `1 * time.Minute` - 1分钟

### 调整策略参数

```go
strategies := []engine.Strategy{
    strategy.NewMACrossStrategy(5, 20),   // 5日和20日均线
    strategy.NewMACrossStrategy(10, 50),  // 10日和50日均线
}
```

参数说明:
- 第一个数字: 短期均线周期
- 第二个数字: 长期均线周期
- 差距越大,信号越滞后但越可靠

### 设置初始持仓

如果你已经持有某些股票,可以在启动时设置:

```go
// 取消注释这些行
eng.UpdatePosition("AAPL", 100, 150.0)   // 持有100股AAPL,平均成本$150
eng.UpdatePosition("GOOGL", 50, 140.0)   // 持有50股GOOGL,平均成本$140
```

## 理解信号

### 买入信号 (BUY)

当出现"金叉"(Golden Cross)时生成:
- 短期均线上穿长期均线
- 表示股票可能进入上涨趋势
- 建议买入

### 卖出信号 (SELL)

当出现"死叉"(Death Cross)时生成:
- 短期均线下穿长期均线
- 表示股票可能进入下跌趋势
- 建议卖出

### 信号置信度

- **70%+**: 强信号,均线差距明显
- **50-70%**: 中等信号,可以考虑
- **30-50%**: 弱信号,需谨慎

## 注意事项

1. **模拟数据**: 当前使用模拟数据,价格是随机生成的
2. **仅供参考**: 信号仅供参考,不构成投资建议
3. **风险自负**: 实际交易前请做好风险评估
4. **需要数据**: 生产环境需要接入真实市场数据

## 常见问题

### Q: 为什么没有信号生成?

A: 可能原因:
- 需要积累足够的历史数据(至少20个数据点)
- 当前没有发生均线交叉
- 等待更长时间让数据累积

### Q: 可以直接执行交易吗?

A: 不可以。系统只生成信号,需要手动或通过券商API执行交易。

### Q: 如何接入真实数据?

A: 需要实现 `provider.DataProvider` 接口,连接真实的数据源API。

### Q: 策略可靠吗?

A: MA交叉是经典策略,但不保证盈利。建议:
- 在模拟环境测试
- 结合其他指标
- 做好风险管理

## 下一步

1. 观察系统运行,理解信号生成逻辑
2. 修改参数,测试不同的策略配置
3. 学习如何添加自定义策略
4. 考虑接入真实数据源

## 获取帮助

查看详细文档:
- `pkg/trading/README.md` - 完整系统文档
- `examples/trading/README.md` - 示例和配置
- `examples/trading/config.example.yaml` - 配置示例

祝交易顺利! 📈
