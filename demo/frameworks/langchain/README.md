# LangChain demo

Records tool spans through `GustCallbackHandler` and is driven by Mode 3. The scenario folder is the composable layout: thin `scenario.yaml` plus `fixtures/*.json`.

```bash
./gust test demo/frameworks/langchain --samples 2
```

`langchain-core` is optional. Without it the same callback methods still record a valid `AgentRun`.
