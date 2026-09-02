# 《东方快车谋杀案》StarGraph GraphRAG vs 传统 RAG 可视化推理系统

这是一个纯前端零依赖的交互式可视化演示系统，直观呈现 **StarGraph (GraphRAG / DRIFT)** 如何突破传统向量检索（Vector RAG）在多跳推理、闭环不在场证明排查和全局共谋推演上的致命死穴。

---

## 🌟 核心特性与界面结构

1. **原著三部曲叙事流 (Story Narrative Chunks)**：
   - 完整还原阿加莎·克里斯蒂原著与维基百科的 6 个分块（案发现场 $\to$ 逐人审讯 A/B/C $\to$ 阿姆斯特朗案揭秘 $\to$ 十二人陪审团终局）。
2. **辛普伦-东方快车加莱车厢平面图 (Calais Coach Map)**：
   - 交互式 16 间包厢平面图，点击任意包厢即时查看嫌疑人表面身份、隐藏真实身份与车厢位置。
3. **力导向交互式知识图谱 (Interactive Force Graph)**：
   - **表面迷雾视角 (Surface Mode / 传统 RAG 视角)**：呈现孤立分散的角色与被虚假不在场证明环路蒙蔽的状态。
   - **图谱真相视角 (Truth Graph / GraphRAG 视角)**：金色点亮阿姆斯特朗核心悲剧，12 根复仇红线直指死者卡塞蒂。
   - **DRIFT 动态穿梭模式 (DRIFT Step Traversal)**：流光动画分步呈现大模型沿拓扑链路的穿梭推理过程。
4. **双栏对比推理控制台 (Side-by-Side Arena)**：
   - 内置三大经典对抗关卡（玛丽隐藏动机、闭环谎言网络、全车共谋真相），支持一键执行与自定义输入。
   - 支持 Mock 预计算结果与本地真实 API 切换。

---

## 🚀 运行与体验方式

### 方式 1：浏览器直接打开（无需启动任何服务）
直接在任意现代浏览器中打开：
```bash
# 浏览器打开 index.html 即可使用全部交互与图谱动画
file:///home/qtopierw/workspace/projects/star-graph/examples/orient_express_demo/index.html
```

### 方式 2：使用任何静态 HTTP 服务器（如 Python / Go / Nginx）
```bash
cd examples/orient_express_demo
python3 -m http.server 8080
# 访问 http://localhost:8080
```

---

## 📊 预分析数据集文件
- **知识图谱与对比数据**：`examples/orient_express_demo/data/orient_express_graph.json`
- **原著叙事语料 Fixture**：`harness/fixtures/orient_express.json`
- **自动化测试用例**：`testings/benchmark_cases/orient_express_benchmark_test.go`
