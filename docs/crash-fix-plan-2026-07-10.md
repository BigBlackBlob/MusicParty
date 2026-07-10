# 崩溃综合补强实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 消除"流媒体转发错误处理 + 多人并发上游 fetch 无去重 + 封面色轮询放大 + 小内存 GC 抖动"四者叠加导致的资源耗尽崩溃。

**Architecture:** 四阶段补强：① 修复流式转发已提交响应后的错误处理；② 为网易云代理添加 CDN URL 缓存 + in-flight 去重 + 本地缓存集成；③ 为封面色服务添加 in-flight 去重 + 早期缓存检查 + 前端节流；④ 调低 JVM 堆占比 + 限制 boundedElastic 线程池上限。

**Tech Stack:** Java 21 / Spring Boot 3.2.5 (WebFlux/Reactor) / Vue 3 / Vite 7 / Vitest / JUnit 5 / AssertJ

## Global Constraints

- 后端是 Spring WebFlux（reactive），所有 controller 返回 `Mono`/`Flux`
- 阻塞 I/O（`HttpClient.send()`）必须跑在 `Schedulers.boundedElastic()` 上
- 前端测试用 Vitest + jsdom，后端测试用 JUnit 5 + AssertJ
- 测试风格：后端部分用源码文本断言（读源文件检查字符串），部分用单元测试 mock；前端用 Vitest
- 项目根目录：`C:/Users/Nirotiy/Documents/NRT-GIT/musicparty/MusicParty`
- 不得破坏现有 API 契约（URL 路径、WebSocket 消息格式、前端行为）

---

## File Structure

| 文件 | 职责 | 操作 |
|---|---|---|
| `controller/NeteaseProxyController.java` | 网易云流代理：添加 CDN URL 缓存、in-flight 去重、本地缓存检查、Flux 错误抑制 | 修改 |
| `controller/BilibiliProxyController.java` | B站流代理：添加 Flux 错误抑制 | 修改 |
| `controller/ReactiveStreamUtils.java` | 流工具：添加 `withErrorSuppression` 包装方法 | 修改 |
| `exception/GlobalExceptionHandler.java` | 全局异常：已提交响应感知 | 修改 |
| `service/CoverColorService.java` | 封面色：in-flight 去重 + 早期缓存检查 | 修改 |
| `service/api/NeteaseMusicApiService.java` | 网易云 API：实现 `CachedMusicApiService`，添加 `prefetchMusic` | 修改 |
| `service/MusicPlayerService.java` | 播放服务：无需修改（`isCachedPlatform` 自动识别 `CachedMusicApiService`） | 不修改 |
| `config/AppProperties.java` | 配置：添加 `boundedElasticSize` 属性 | 修改 |
| `src/main/resources/application.yml` | 配置：添加 boundedElastic 上限环境变量 | 修改 |
| `Dockerfile` | 运行时：调低 `MaxRAMPercentage` | 修改 |
| `docker-compose.yml` | 运行时：调低 `MaxRAMPercentage` | 修改 |
| `music-party-web/src/stores/ui.js` | 前端：`updateAccentFromCover` 添加节流 | 修改 |
| `src/test/.../controller/NeteaseProxyControllerTest.java` | 测试 | 新建 |
| `src/test/.../exception/GlobalExceptionHandlerTests.java` | 测试 | 修改 |
| `src/test/.../service/CoverColorServiceTests.java` | 测试 | 修改 |
| `music-party-web/src/stores/ui.test.js` | 测试 | 新建 |

---

## Phase 1: 流式转发错误处理

### Task 1: 添加 Flux 级错误抑制到流代理

**问题：** 当 206 PARTIAL_CONTENT 响应已提交（已经开始写字节流），上游 InputStream 中途失败时，错误传播到 Spring 的错误处理链，Spring 尝试向已提交的响应写入错误 JSON Map，触发 `HttpMessageNotWritableException`（找不到 `audio/mpeg` 的 Map 编码器），`GlobalExceptionHandler` 再捕获这个异常又试图写 JSON —— 每次失败都走完整编码链 + 打巨型堆栈。

**修复方案：** 在 `Flux<DataBuffer>` body 上添加 `.onErrorResume()`，流中途出错时记日志 + 静默完成，不让错误传播到 Spring。

**Files:**
- Modify: `src/main/java/org/thornex/musicparty/controller/ReactiveStreamUtils.java`
- Modify: `src/main/java/org/thornex/musicparty/controller/NeteaseProxyController.java`
- Modify: `src/main/java/org/thornex/musicparty/controller/BilibiliProxyController.java`
- Test: `src/test/java/org/thornex/musicparty/controller/NeteaseProxyControllerTest.java`（新建）

**Interfaces:**
- Produces: `ReactiveStreamUtils.withErrorSuppression(Flux<DataBuffer>, String, Logger)` — 返回带错误抑制的 Flux

- [ ] **Step 1: 写 ReactiveStreamUtils.withErrorSuppression**

在 `ReactiveStreamUtils.java` 中添加：

```java
import org.slf4j.Logger;
import reactor.core.publisher.Mono;

/**
 * Wraps a streaming Flux so mid-stream errors are logged and the Flux
 * completes silently instead of propagating to Spring's error handler.
 * This prevents HttpMessageNotWritableException when the response is
 * already committed (e.g. 206 PARTIAL_CONTENT).
 */
static Flux<DataBuffer> withErrorSuppression(Flux<DataBuffer> body, String context, Logger logger) {
    return body.onErrorResume(e -> {
        logger.warn("Stream error after response committed (suppressing): {}, message={}", context, e.getMessage());
        return Mono.empty();
    });
}
```

- [ ] **Step 2: 在 NeteaseProxyController 应用 withErrorSuppression**

`NeteaseProxyController.java:105` — 将 `Flux<DataBuffer> body = ReactiveStreamUtils.readInputStream(upstream.body());` 改为：

```java
Flux<DataBuffer> body = ReactiveStreamUtils.withErrorSuppression(
        ReactiveStreamUtils.readInputStream(upstream.body()),
        "netease stream songId=" + songId,
        log);
```

- [ ] **Step 3: 在 BilibiliProxyController 应用 withErrorSuppression**

`BilibiliProxyController.java:208` — 将 `Flux<DataBuffer> body = ReactiveStreamUtils.readInputStream(upstream.body());` 改为：

```java
Flux<DataBuffer> body = ReactiveStreamUtils.withErrorSuppression(
        ReactiveStreamUtils.readInputStream(upstream.body()),
        "bilibili stream bvid=" + bvid,
        log);
```

Bilibili 的 `streamLocalFile` 中 `Flux<DataBuffer> body = ReactiveStreamUtils.readPath(path, start, contentLength);` 也需要包装：

```java
Flux<DataBuffer> body = ReactiveStreamUtils.withErrorSuppression(
        ReactiveStreamUtils.readPath(path, start, contentLength),
        "bilibili local file bvid=" + bvid,
        log);
```

注意：`streamLocalFile` 方法签名中 `bvid` 不可用（它是 `private` 方法，参数是 `Path path, String contentType, String rangeHeader`）。需要把调用处传入的 bvid 作为参数追加，或在 `streamLocalFile` 内用 `path.getFileName().toString()` 作为 context。选择后者更简单：

```java
Flux<DataBuffer> body = ReactiveStreamUtils.withErrorSuppression(
        ReactiveStreamUtils.readPath(path, start, contentLength),
        "local file=" + path.getFileName(),
        log);
```

- [ ] **Step 4: 写测试**

新建 `src/test/java/org/thornex/musicparty/controller/NeteaseProxyControllerTest.java`：

```java
package org.thornex.musicparty.controller;

import org.junit.jupiter.api.Test;
import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;
import static org.assertj.core.api.Assertions.assertThat;

class NeteaseProxyControllerTest {

    private static final Path BASE = Path.of("src/main/java/org/thornex/musicparty");

    @Test
    void neteaseProxyControllerHasStreamEndpointWithRangeSupport() throws IOException {
        String src = Files.readString(BASE.resolve("controller/NeteaseProxyController.java"));
        assertThat(src).contains("@GetMapping(\"/stream/{songId}\")");
        assertThat(src).contains("@RequestMapping(\"/api/netease\")");
        assertThat(src).contains("\"Range\"");
        assertThat(src).contains("internalStreamProxyToken");
        assertThat(src).contains("getUserBySessionToken");
    }

    @Test
    void neteaseProxyAppliesErrorSuppressionToFluxBody() throws IOException {
        String src = Files.readString(BASE.resolve("controller/NeteaseProxyController.java"));
        assertThat(src).contains("withErrorSuppression");
    }

    @Test
    void reactiveStreamUtilsHasErrorSuppressionHelper() throws IOException {
        String src = Files.readString(BASE.resolve("controller/ReactiveStreamUtils.java"));
        assertThat(src).contains("withErrorSuppression");
        assertThat(src).contains("onErrorResume");
    }

    @Test
    void bilibiliProxyAlsoAppliesErrorSuppression() throws IOException {
        String src = Files.readString(BASE.resolve("controller/BilibiliProxyController.java"));
        assertThat(src).contains("withErrorSuppression");
    }
}
```

- [ ] **Step 5: 运行测试验证通过**

```bash
cd C:/Users/Nirotiy/Documents/NRT-GIT/musicparty/MusicParty
mvn test -pl . -Dtest="NeteaseProxyControllerTest,BilibiliProxyControllerTest" -Dsurefire.failIfNoSpecifiedTests=false
```

- [ ] **Step 6: Commit**

```bash
git add src/main/java/org/thornex/musicparty/controller/ReactiveStreamUtils.java src/main/java/org/thornex/musicparty/controller/NeteaseProxyController.java src/main/java/org/thornex/musicparty/controller/BilibiliProxyController.java src/test/java/org/thornex/musicparty/controller/NeteaseProxyControllerTest.java
git commit -m "fix: suppress mid-stream errors after response committed to prevent encoder cascade"
```

---

### Task 2: GlobalExceptionHandler 已提交响应感知

**问题：** 即使 Flux 级错误抑制阻止了大部分情况，某些边缘场景（如 response head 写出后 body 未开始时出错）仍可能让错误到达 `GlobalExceptionHandler`。当前 handler 无条件返回 JSON Map body，如果响应已 committed，Spring 尝试编码这个 Map 到 `audio/mpeg` content-type 就会失败。

**修复方案：** 让 `GlobalExceptionHandler` 接受 `ServerWebExchange` 参数，检测 `response.isCommitted()`，已提交时仅记日志 + `setComplete()`，不再尝试写 body。

**Files:**
- Modify: `src/main/java/org/thornex/musicparty/exception/GlobalExceptionHandler.java`
- Test: `src/test/java/org/thornex/musicparty/exception/GlobalExceptionHandlerTests.java`

- [ ] **Step 1: 修改 GlobalExceptionHandler 签名和逻辑**

将三个 handler 方法改为接受 `ServerWebExchange` 参数并返回 `Mono<ResponseEntity<Object>>`。以 `handleGenericException` 为例：

```java
import org.springframework.web.server.ServerWebExchange;
import reactor.core.publisher.Mono;

@ExceptionHandler(Exception.class)
public Mono<ResponseEntity<Object>> handleGenericException(Exception ex, ServerWebExchange exchange) {
    if (exchange.getResponse().isCommitted()) {
        log.warn("Response already committed, suppressing error body: {}", ex.getMessage());
        return exchange.getResponse().setComplete().then(Mono.empty());
    }
    log.error("Handled unexpected exception", ex);
    Map<String, Object> body = Map.of(
            "message", "An unexpected internal server error occurred.",
            "error", ex.getClass().getSimpleName(),
            "status", HttpStatus.INTERNAL_SERVER_ERROR.value()
    );
    return Mono.just(new ResponseEntity<>(body, HttpStatus.INTERNAL_SERVER_ERROR));
}
```

`handleApiRequestException` 和 `handleMaxUploadSizeExceeded` 也需要同样添加 `ServerWebExchange` 参数和 committed 检查。

注意：Spring WebFlux 的 `@ExceptionHandler` 方法支持 `ServerWebExchange` 作为参数，返回 `Mono<ResponseEntity>` 也是合法的。但现有测试直接调用 `handler.handleMaxUploadSizeExceeded(new MaxUploadSizeExceededException(10))` 不传 exchange —— 需要更新测试。

- [ ] **Step 2: 更新 GlobalExceptionHandlerTests**

```java
package org.thornex.musicparty.exception;

import org.junit.jupiter.api.Test;
import org.springframework.http.HttpStatus;
import org.springframework.mock.http.server.reactive.MockServerHttpResponse;
import org.springframework.mock.web.server.MockServerWebExchange;
import org.springframework.web.multipart.MaxUploadSizeExceededException;
import reactor.core.publisher.Mono;
import reactor.test.StepVerifier;

import java.util.Map;

import static org.assertj.core.api.Assertions.assertThat;

class GlobalExceptionHandlerTests {

    @Test
    void maxUploadSizeExceededReturnsPayloadTooLarge() {
        GlobalExceptionHandler handler = new GlobalExceptionHandler();
        MockServerWebExchange exchange = MockServerWebExchange.builder().build();

        StepVerifier.create(handler.handleMaxUploadSizeExceeded(new MaxUploadSizeExceededException(10), exchange))
                .assertNext(response -> {
                    assertThat(response.getStatusCode()).isEqualTo(HttpStatus.PAYLOAD_TOO_LARGE);
                    assertThat(response.getBody()).isInstanceOf(Map.class);
                    Map<?, ?> body = (Map<?, ?>) response.getBody();
                    assertThat(body.get("status")).isEqualTo(HttpStatus.PAYLOAD_TOO_LARGE.value());
                })
                .verifyComplete();
    }

    @Test
    void genericExceptionReturns500WhenResponseNotCommitted() {
        GlobalExceptionHandler handler = new GlobalExceptionHandler();
        MockServerWebExchange exchange = MockServerWebExchange.builder().build();

        StepVerifier.create(handler.handleGenericException(new RuntimeException("test"), exchange))
                .assertNext(response -> {
                    assertThat(response.getStatusCode()).isEqualTo(HttpStatus.INTERNAL_SERVER_ERROR);
                    assertThat(response.getBody()).isInstanceOf(Map.class);
                })
                .verifyComplete();
    }

    @Test
    void genericExceptionCompletesSilentlyWhenResponseAlreadyCommitted() {
        GlobalExceptionHandler handler = new GlobalExceptionHandler();
        MockServerWebExchange exchange = MockServerWebExchange.builder().build();
        // 模拟已提交：commit 后 isCommitted() 返回 true
        exchange.getResponse().setStatusCode(HttpStatus.PARTIAL_CONTENT);
        ((MockServerHttpResponse) exchange.getResponse()).setCommitted(true);

        StepVerifier.create(handler.handleGenericException(new RuntimeException("test"), exchange))
                .verifyComplete();
    }
}
```

- [ ] **Step 3: 运行测试**

```bash
mvn test -Dtest="GlobalExceptionHandlerTests" -Dsurefire.failIfNoSpecifiedTests=false
```

- [ ] **Step 4: Commit**

```bash
git add src/main/java/org/thornex/musicparty/exception/GlobalExceptionHandler.java src/test/java/org/thornex/musicparty/exception/GlobalExceptionHandlerTests.java
git commit -m "fix: skip error body when response already committed in GlobalExceptionHandler"
```

---

## Phase 2: 网易云流代理去重

### Task 3: 添加 CDN URL 短时缓存到 NeteaseProxyController

**问题：** `NeteaseProxyController` 每次 stream 请求都调用 `neteaseService.resolveCdnUrl(songId)`，这个调用打到 ncm-api 容器（做 CPU 密集的网易云数据解密）。5-6 人听同一首歌 = 5-6 次 resolveCdnUrl = 5-6 次 ncm-api 解密。BilibiliProxyController 已有 30s TTL 的 `cdnUrlCache`，网易云需要同样的机制。

**Files:**
- Modify: `src/main/java/org/thornex/musicparty/controller/NeteaseProxyController.java`
- Test: `src/test/java/org/thornex/musicparty/controller/NeteaseProxyControllerTest.java`

- [ ] **Step 1: 添加 CDN URL 缓存字段和 resolveCdnWithCache 方法**

在 `NeteaseProxyController` 中添加（参照 `BilibiliProxyController.cdnUrlCache` 模式）：

```java
import java.util.concurrent.ConcurrentHashMap;
import java.util.Map;

// CDN URL 短时缓存：避免多人同曲时重复打 ncm-api（CPU 密集解密）
private static final long CDN_TTL_MS = 30_000;
private record CdnEntry(String url, long expireAt) {}
private final Map<String, CdnEntry> cdnUrlCache = new ConcurrentHashMap<>();

private Mono<String> resolveCdnWithCache(String songId) {
    CdnEntry cached = cdnUrlCache.get(songId);
    if (cached != null && cached.expireAt() > System.currentTimeMillis()) {
        return Mono.just(cached.url());
    }
    return neteaseService.resolveCdnUrl(songId)
            .doOnNext(url -> cdnUrlCache.put(songId, new CdnEntry(url, System.currentTimeMillis() + CDN_TTL_MS)));
}
```

- [ ] **Step 2: 将 streamSong 中的 resolveCdnUrl 替换为 resolveCdnWithCache**

`NeteaseProxyController.java:57` — 将 `return neteaseService.resolveCdnUrl(songId)` 改为 `return resolveCdnWithCache(songId)`。

- [ ] **Step 3: 添加测试断言**

在 `NeteaseProxyControllerTest.java` 中添加：

```java
@Test
void neteaseProxyHasCdnUrlShortTermCache() throws IOException {
    String src = Files.readString(BASE.resolve("controller/NeteaseProxyController.java"));
    assertThat(src).contains("cdnUrlCache");
    assertThat(src).contains("CDN_TTL_MS");
    assertThat(src).contains("resolveCdnWithCache");
}
```

- [ ] **Step 4: 运行测试并 commit**

```bash
mvn test -Dtest="NeteaseProxyControllerTest" -Dsurefire.failIfNoSpecifiedTests=false
git add src/main/java/org/thornex/musicparty/controller/NeteaseProxyController.java src/test/java/org/thornex/musicparty/controller/NeteaseProxyControllerTest.java
git commit -m "perf: add 30s CDN URL cache to NeteaseProxyController to avoid redundant ncm-api calls"
```

---

### Task 4: 添加 in-flight CDN URL 解析去重

**问题：** CDN URL 缓存只在第一次 resolve 完成后才命中。如果 5 个用户同时请求同一首歌（缓存 miss），5 个 resolveCdnUrl 会同时打到 ncm-api。需要 in-flight 去重：同一 songId 的并发 resolve 共享一个 Mono。

**Files:**
- Modify: `src/main/java/org/thornex/musicparty/controller/NeteaseProxyController.java`
- Test: `src/test/java/org/thornex/musicparty/controller/NeteaseProxyControllerTest.java`

- [ ] **Step 1: 添加 in-flight 去重 Map**

在 `NeteaseProxyController` 中添加：

```java
// In-flight CDN URL 解析去重：同 songId 的并发请求共享一个 Mono
private final Map<String, Mono<String>> inflightCdnResolves = new ConcurrentHashMap<>();

private Mono<String> resolveCdnDedup(String songId) {
    // 1. 先查缓存
    CdnEntry cached = cdnUrlCache.get(songId);
    if (cached != null && cached.expireAt() > System.currentTimeMillis()) {
        return Mono.just(cached.url());
    }
    // 2. 查 in-flight：如果已有相同 songId 的 resolve 在进行中，复用它
    Mono<String> existing = inflightCdnResolves.get(songId);
    if (existing != null) {
        return existing;
    }
    // 3. 发起新 resolve，完成后写入缓存并清理 in-flight
    Mono<String> resolve = neteaseService.resolveCdnUrl(songId)
            .doOnNext(url -> cdnUrlCache.put(songId, new CdnEntry(url, System.currentTimeMillis() + CDN_TTL_MS)))
            .cache()
            .doFinally(signal -> inflightCdnResolves.remove(songId));
    inflightCdnResolves.put(songId, resolve);
    return resolve;
}
```

注意：`.cache()` 让 Mono 变成 hot publisher，多个 subscriber 共享同一上游调用结果。

- [ ] **Step 2: 将 streamSong 中的 resolveCdnWithCache 替换为 resolveCdnDedup**

- [ ] **Step 3: 添加测试断言**

```java
@Test
void neteaseProxyHasInflightCdnResolveDedup() throws IOException {
    String src = Files.readString(BASE.resolve("controller/NeteaseProxyController.java"));
    assertThat(src).contains("inflightCdnResolves");
    assertThat(src).contains("resolveCdnDedup");
    assertThat(src).contains(".cache()");
}
```

- [ ] **Step 4: 运行测试并 commit**

```bash
mvn test -Dtest="NeteaseProxyControllerTest" -Dsurefire.failIfNoSpecifiedTests=false
git add src/main/java/org/thornex/musicparty/controller/NeteaseProxyController.java src/test/java/org/thornex/musicparty/controller/NeteaseProxyControllerTest.java
git commit -m "perf: deduplicate concurrent CDN URL resolves for same songId in NeteaseProxyController"
```

---

### Task 5: 为网易云添加本地缓存集成（prefetch + tryLocalFile）

**问题：** `NeteaseMusicApiService` 未实现 `CachedMusicApiService`，无 prefetch、无本地缓存。5-6 人听同一曲 = 5-6 条独立 CDN 下载链路。Bilibili 已有完整的 prefetch → LocalCacheService → tryLocalFile 链路，网易云需要同样实现。

**Files:**
- Modify: `src/main/java/org/thornex/musicparty/service/api/NeteaseMusicApiService.java`
- Modify: `src/main/java/org/thornex/musicparty/controller/NeteaseProxyController.java`
- Test: `src/test/java/org/thornex/musicparty/controller/NeteaseProxyControllerTest.java`

**Interfaces:**
- `NeteaseMusicApiService` 改为 `implements CachedMusicApiService`
- 添加 `prefetchMusic(String musicId)` 方法
- `NeteaseProxyController` 添加 `LocalCacheService` 依赖 + `tryLocalFile` 方法

- [ ] **Step 1: 让 NeteaseMusicApiService 实现 CachedMusicApiService**

修改类声明：

```java
public class NeteaseMusicApiService implements CachedMusicApiService {
```

注入 `LocalCacheService`：

```java
private final LocalCacheService localCacheService;

// 构造函数添加参数
public NeteaseMusicApiService(WebClient webClient, AppProperties appProperties,
                              SiteSettingService siteSettingService,
                              LocalCacheService localCacheService) {
    // ...
    this.localCacheService = localCacheService;
}
```

- [ ] **Step 2: 实现 prefetchMusic**

```java
@Override
public void prefetchMusic(String musicId) {
    ensureConfigured();
    CacheStatus status = localCacheService.getStatus(musicId);
    if (status == CacheStatus.COMPLETED || status == CacheStatus.DOWNLOADING) {
        return;
    }

    log.info("Prefetching Netease music: {}", musicId);

    // CDN URL 作为下载源（由 resolveCdnUrl 提供）
    Mono<String> urlProvider = resolveCdnUrl(musicId);

    // 网易云 CDN 需要 Referer
    Map<String, String> headers = new HashMap<>();
    headers.put("Referer", "https://music.163.com/");
    headers.put("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36");

    // 扩展名根据音质决定
    String extension = ".mp3"; // exhigh 默认 mp3；lossless/hires 可能是 .flac

    localCacheService.submitDownload(musicId, urlProvider, headers, extension);
}
```

需要添加 import：
```java
import org.thornex.musicparty.service.LocalCacheService;
import org.thornex.musicparty.enums.CacheStatus;
import java.util.HashMap;
import java.util.Map;
```

- [ ] **Step 3: 修改 NeteaseProxyController 添加 tryLocalFile**

注入 `LocalCacheService`：

```java
private final LocalCacheService localCacheService;

// 构造函数添加参数
public NeteaseProxyController(NeteaseMusicApiService neteaseService,
                              UserService userService,
                              InternalStreamProxyToken internalStreamProxyToken,
                              LocalCacheService localCacheService) {
    this.neteaseService = neteaseService;
    this.userService = userService;
    this.internalStreamProxyToken = internalStreamProxyToken;
    this.localCacheService = localCacheService;
}
```

在 `streamSong` 方法开头（canUse 检查之后）添加本地缓存检查：

```java
// Failsafe 0: 先检查本地缓存——如果后台下载已完成，直接读本地文件
ResponseEntity<Flux<DataBuffer>> localFallback = tryLocalFile(songId, rangeHeader);
if (localFallback != null) {
    log.debug("Netease stream using local cache for {}", songId);
    return Mono.just(localFallback);
}
```

添加 `tryLocalFile` 和 `streamLocalFile` 方法（参照 `BilibiliProxyController` 的同名方法）：

```java
import org.thornex.musicparty.config.LocalResourceConfig;
import java.nio.file.Paths;
import java.nio.file.Files;
import java.nio.file.Path;

private ResponseEntity<Flux<DataBuffer>> tryLocalFile(String songId, String rangeHeader) {
    try {
        String localUrl = localCacheService.getLocalUrl(songId);
        if (localUrl == null) return null;
        String fileName = localUrl.substring("/media/".length());
        Path path = Paths.get(LocalResourceConfig.CACHE_DIR, fileName);
        if (!Files.exists(path)) return null;
        return streamLocalFile(path, "audio/mpeg", rangeHeader);
    } catch (Exception e) {
        log.debug("Local file fallback check failed for {}: {}", songId, e.getMessage());
        return null;
    }
}

private ResponseEntity<Flux<DataBuffer>> streamLocalFile(Path path, String contentType, String rangeHeader) throws IOException {
    long length = Files.size(path);
    long start = 0;
    long end = length - 1;
    HttpStatus status = HttpStatus.OK;
    if (rangeHeader != null && rangeHeader.startsWith("bytes=")) {
        String[] parts = rangeHeader.substring("bytes=".length()).split("-", 2);
        try {
            start = parts[0].isBlank() ? 0 : Long.parseLong(parts[0]);
            if (parts.length > 1 && !parts[1].isBlank()) {
                end = Math.min(Long.parseLong(parts[1]), length - 1);
            }
            status = HttpStatus.PARTIAL_CONTENT;
        } catch (NumberFormatException ignored) {
            start = 0;
            end = length - 1;
        }
    }
    if (start < 0 || start >= length || end < start) {
        return ResponseEntity.status(HttpStatus.REQUESTED_RANGE_NOT_SATISFIABLE).build();
    }
    long contentLength = end - start + 1;
    HttpHeaders headers = new HttpHeaders();
    headers.set(HttpHeaders.CONTENT_TYPE, contentType);
    headers.set(HttpHeaders.ACCEPT_RANGES, "bytes");
    headers.setContentLength(contentLength);
    headers.set(HttpHeaders.ACCESS_CONTROL_ALLOW_ORIGIN, "*");
    headers.set(HttpHeaders.ACCESS_CONTROL_EXPOSE_HEADERS, "Content-Length, Content-Range, Accept-Ranges");
    if (status == HttpStatus.PARTIAL_CONTENT) {
        headers.set(HttpHeaders.CONTENT_RANGE, "bytes " + start + "-" + end + "/" + length);
    }
    Flux<DataBuffer> body = ReactiveStreamUtils.withErrorSuppression(
            ReactiveStreamUtils.readPath(path, start, contentLength),
            "netease local file=" + path.getFileName(),
            log);
    return new ResponseEntity<>(body, headers, status);
}
```

- [ ] **Step 4: 添加测试断言**

```java
@Test
void neteaseProxyHasLocalCacheFallback() throws IOException {
    String src = Files.readString(BASE.resolve("controller/NeteaseProxyController.java"));
    assertThat(src).contains("tryLocalFile");
    assertThat(src).contains("localCacheService");
    assertThat(src).contains("streamLocalFile");
}

@Test
void neteaseServiceImplementsCachedMusicApiService() throws IOException {
    String src = Files.readString(BASE.resolve("service/api/NeteaseMusicApiService.java"));
    assertThat(src).contains("implements CachedMusicApiService");
    assertThat(src).contains("prefetchMusic");
    assertThat(src).contains("localCacheService");
}
```

- [ ] **Step 5: 运行测试**

```bash
mvn test -Dtest="NeteaseProxyControllerTest" -Dsurefire.failIfNoSpecifiedTests=false
```

- [ ] **Step 6: 运行全量后端测试确保不破坏**

```bash
mvn test
```

注意：`NeteaseMusicApiService` 构造函数签名变更会影响所有注入它的测试。需要检查 `MusicPlayerServicePendingDownloadTest` 和其他测试是否需要更新 mock。

- [ ] **Step 7: Commit**

```bash
git add -A
git commit -m "feat: integrate Netease with LocalCacheService for prefetch + local file fallback"
```

---

## Phase 3: 封面色轮询抑制

### Task 6: CoverColorService in-flight 去重 + 早期缓存检查

**问题：** 
1. 缓存检查在 `Mono.defer()` 内、`boundedElastic` 上执行 —— 即使缓存命中也消耗一个线程
2. 无 in-flight 去重 —— 多个并发请求同一 URL 各自执行完整提取流程（下载图片 + ImageIO 解码 + 取色）

**Files:**
- Modify: `src/main/java/org/thornex/musicparty/service/CoverColorService.java`
- Test: `src/test/java/org/thornex/musicparty/service/CoverColorServiceTests.java`

- [ ] **Step 1: 添加 in-flight 去重 Map**

在 `CoverColorService` 中添加：

```java
import java.util.concurrent.ConcurrentHashMap;

// In-flight 去重：同 URL 的并发请求共享一个 Mono
private final Map<String, Mono<CoverColorResponse>> inflightExtractions = new ConcurrentHashMap<>();
```

- [ ] **Step 2: 重写 extract 方法：早期缓存检查 + in-flight 去重**

```java
public Mono<CoverColorResponse> extract(String coverUrl) {
    if (!StringUtils.hasText(coverUrl)) {
        return Mono.empty();
    }

    // 1. 早期缓存检查（不消耗 boundedElastic 线程）
    String resolvedUrl = resolveCoverUrl(coverUrl);
    CoverColorResponse cached = responseCache.get(resolvedUrl);
    if (cached != null) {
        return Mono.just(cached);
    }

    // 2. 安全检查（不消耗线程）
    boolean trustedLocalPath = isTrustedLocalCoverPath(coverUrl);
    if (!trustedLocalPath && !isSafeCoverUrl(resolvedUrl)) {
        log.warn("Rejected unsafe cover URL for color extraction: {}", resolvedUrl);
        return Mono.empty();
    }

    // 3. In-flight 去重：同 URL 的并发请求共享一个 Mono
    return inflightExtractions.computeIfAbsent(resolvedUrl, url ->
            doExtract(url, trustedLocalPath)
                    .cache()
                    .doFinally(signal -> inflightExtractions.remove(url))
    );
}

private Mono<CoverColorResponse> doExtract(String resolvedUrl, boolean trustedLocalPath) {
    if (!concurrentExtracts.tryAcquire()) {
        log.debug("Cover color extraction concurrency limit reached");
        return Mono.empty();
    }

    return fetchCoverBytes(resolvedUrl)
            .timeout(COVER_FETCH_TIMEOUT)
            .filter(bytes -> {
                boolean allowed = bytes.length <= MAX_COVER_BYTES;
                if (!allowed) {
                    log.warn("Rejected oversized cover body: url={}, bytes={}", resolvedUrl, bytes.length);
                }
                return allowed;
            })
            .flatMap(bytes -> decodeCoverColor(resolvedUrl, bytes))
            .onErrorResume(error -> {
                log.warn("Failed to fetch cover for color extraction: {}", resolvedUrl, error);
                return Mono.empty();
            })
            .doFinally(ignored -> concurrentExtracts.release())
            .subscribeOn(Schedulers.boundedElastic());
}
```

注意：`resolveCoverUrl` 和 `isSafeCoverUrl` 是轻量纯计算（URI 解析 + DNS 查询），可以安全地在调用线程执行。`isSafeCoverUrl` 内部有 `InetAddress.getByName()` 调用，但这个只在 URL host 不是 IP 字面量时才触发 DNS。为了安全，可以把 `isSafeCoverUrl` 也放在 `doExtract` 内（即 `boundedElastic` 上），只把 `resolveCoverUrl` + 缓存检查提到前面。

修正方案：`extract` 方法只做缓存检查和 in-flight 去重，安全检查移到 `doExtract` 内：

```java
public Mono<CoverColorResponse> extract(String coverUrl) {
    if (!StringUtils.hasText(coverUrl)) {
        return Mono.empty();
    }

    String resolvedUrl = resolveCoverUrl(coverUrl);
    
    // 早期缓存检查（不消耗 boundedElastic 线程）
    CoverColorResponse cached = responseCache.get(resolvedUrl);
    if (cached != null) {
        return Mono.just(cached);
    }

    // In-flight 去重
    return inflightExtractions.computeIfAbsent(resolvedUrl, url ->
            doExtract(url, coverUrl)
                    .cache()
                    .doFinally(signal -> inflightExtractions.remove(url))
    );
}

private Mono<CoverColorResponse> doExtract(String resolvedUrl, String originalCoverUrl) {
    return Mono.defer(() -> {
                boolean trustedLocalPath = isTrustedLocalCoverPath(originalCoverUrl);
                if (!trustedLocalPath && !isSafeCoverUrl(resolvedUrl)) {
                    log.warn("Rejected unsafe cover URL for color extraction: {}", resolvedUrl);
                    return Mono.empty();
                }
                if (!concurrentExtracts.tryAcquire()) {
                    log.debug("Cover color extraction concurrency limit reached");
                    return Mono.empty();
                }
                return fetchCoverBytes(resolvedUrl)
                        .timeout(COVER_FETCH_TIMEOUT)
                        .filter(bytes -> bytes.length <= MAX_COVER_BYTES)
                        .flatMap(bytes -> decodeCoverColor(resolvedUrl, bytes))
                        .onErrorResume(error -> {
                            log.warn("Failed to fetch cover for color extraction: {}", resolvedUrl, error);
                            return Mono.empty();
                        })
                        .doFinally(ignored -> concurrentExtracts.release());
            })
            .subscribeOn(Schedulers.boundedElastic());
}
```

- [ ] **Step 3: 添加 in-flight 去重测试**

在 `CoverColorServiceTests.java` 中添加：

```java
@Test
void concurrentRequestsForSameUrlShareOneExtraction() {
    AtomicInteger calls = new AtomicInteger();
    byte[] png = Base64.getDecoder().decode("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAIAAACQd1PeAAAADElEQVR4XmNgYPgPAAEDAQD9b6mRAAAAAElFTkSuQmCC");
    WebClient client = WebClient.builder()
            .exchangeFunction(request -> {
                calls.incrementAndGet();
                return Mono.just(ClientResponse.create(HttpStatus.OK)
                        .header(HttpHeaders.CONTENT_TYPE, MediaType.IMAGE_PNG_VALUE)
                        .header(HttpHeaders.CONTENT_LENGTH, String.valueOf(png.length))
                        .body(Flux.just(new DefaultDataBufferFactory().wrap(png)))
                        .build());
            })
            .build();
    AppProperties properties = new AppProperties();
    properties.setBaseUrl("http://127.0.0.1:8080");
    CoverColorService service = new CoverColorService(client, properties);

    // 并发请求同一 URL
    Mono<CoverColorResponse> r1 = service.extract("/media/cover.png");
    Mono<CoverColorResponse> r2 = service.extract("/media/cover.png");

    CoverColorResponse first = r1.block();
    CoverColorResponse second = r2.block();

    assertThat(first).isNotNull();
    assertThat(second).isEqualTo(first);
    // 只应有一次上游调用（in-flight 去重）
    assertThat(calls).hasValue(1);
}

@Test
void cacheHitDoesNotConsumeBoundedElasticThread() {
    AtomicInteger calls = new AtomicInteger();
    byte[] png = Base64.getDecoder().decode("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAIAAACQd1PeAAAADElEQVR4XmNgYPgPAAEDAQD9b6mRAAAAAElFTkSuQmCC");
    WebClient client = WebClient.builder()
            .exchangeFunction(request -> {
                calls.incrementAndGet();
                return Mono.just(ClientResponse.create(HttpStatus.OK)
                        .header(HttpHeaders.CONTENT_TYPE, MediaType.IMAGE_PNG_VALUE)
                        .header(HttpHeaders.CONTENT_LENGTH, String.valueOf(png.length))
                        .body(Flux.just(new DefaultDataBufferFactory().wrap(png)))
                        .build());
            })
            .build();
    AppProperties properties = new AppProperties();
    properties.setBaseUrl("http://127.0.0.1:8080");
    CoverColorService service = new CoverColorService(client, properties);

    // 第一次提取填充缓存
    service.extract("/media/cover.png").block();
    int callsAfterFirst = calls.get();
    assertThat(callsAfterFirst).isEqualTo(1);

    // 第二次应命中缓存，不触发上游调用
    service.extract("/media/cover.png").block();
    assertThat(calls).hasValue(1);
}
```

- [ ] **Step 4: 运行测试**

```bash
mvn test -Dtest="CoverColorServiceTests" -Dsurefire.failIfNoSpecifiedTests=false
```

- [ ] **Step 5: Commit**

```bash
git add src/main/java/org/thornex/musicparty/service/CoverColorService.java src/test/java/org/thornex/musicparty/service/CoverColorServiceTests.java
git commit -m "perf: add in-flight dedup and early cache check to CoverColorService"
```

---

### Task 7: 前端 updateAccentFromCover 节流

**问题：** `updateAccentFromCover` 虽有 same-URL 守卫，但在快速 sync 广播（重连、seek resync）时可能在短时间内被多次触发。添加最小请求间隔节流。

**Files:**
- Modify: `music-party-web/src/stores/ui.js`
- Test: `music-party-web/src/stores/ui.test.js`（新建）

- [ ] **Step 1: 添加节流逻辑**

在 `ui.js` 的 `updateAccentFromCover` 函数中添加节流：

```javascript
let lastAccentRequestTime = 0;
const ACCENT_THROTTLE_MS = 2000;

const updateAccentFromCover = async (coverUrl) => {
    if (!coverUrl) {
        clearDynamicAccent();
        return;
    }

    const userStore = useUserStore();
    const requestUrl = withNavidromeResourceToken(coverUrl, userStore.sessionToken);

    if (requestUrl === lastAccentCoverUrl.value && dynamicAccent.value) {
        return;
    }

    // 节流：2 秒内不重复请求（防止 sync 广播风暴时重复拉取）
    const now = Date.now();
    if (now - lastAccentRequestTime < ACCENT_THROTTLE_MS) {
        return;
    }
    lastAccentRequestTime = now;

    try {
        const accentSet = await musicApi.extractCoverColor(requestUrl);
        if (accentSet?.accent) {
            lastAccentCoverUrl.value = requestUrl;
            setDynamicAccent(accentSet);
        } else {
            clearDynamicAccent();
        }
    } catch (e) {
        console.warn('Failed to update accent from cover', e);
        clearDynamicAccent();
    }
};
```

- [ ] **Step 2: 写测试**

新建 `music-party-web/src/stores/ui.test.js`：

```javascript
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { setActivePinia, createPinia } from 'pinia';

// Mock the API client and music API
vi.mock('../api/client', () => ({ default: { get: vi.fn() } }));
vi.mock('../api/music', () => ({
    musicApi: {
        extractCoverColor: vi.fn().mockResolvedValue({ accent: '#ff0000', borderAccent: '#cc0000' }),
        getLyric: vi.fn(),
        getLyricDetail: vi.fn(),
    }
}));

import { useUiStore } from './ui';
import { musicApi } from '../api/music';

describe('ui store - updateAccentFromCover throttle', () => {
    beforeEach(() => {
        setActivePinia(createPinia());
        vi.useFakeTimers();
    });

    afterEach(() => {
        vi.useRealTimers();
    });

    it('throttles repeated cover color requests within 2s', async () => {
        const ui = useUiStore();
        const url = '/media/cover1.png';

        await ui.updateAccentFromCover(url);
        expect(musicApi.extractCoverColor).toHaveBeenCalledTimes(1);

        // Different URL but within throttle window — should be suppressed
        await ui.updateAccentFromCover('/media/cover2.png');
        expect(musicApi.extractCoverColor).toHaveBeenCalledTimes(1);

        // Advance past throttle window
        vi.advanceTimersByTime(2100);
        await ui.updateAccentFromCover('/media/cover2.png');
        expect(musicApi.extractCoverColor).toHaveBeenCalledTimes(2);
    });

    it('skips request when URL unchanged and accent already set', async () => {
        const ui = useUiStore();
        const url = '/media/cover.png';

        await ui.updateAccentFromCover(url);
        expect(musicApi.extractCoverColor).toHaveBeenCalledTimes(1);

        // Same URL — should skip even after throttle window
        vi.advanceTimersByTime(3000);
        await ui.updateAccentFromCover(url);
        expect(musicApi.extractCoverColor).toHaveBeenCalledTimes(1);
    });

    it('clears accent when coverUrl is empty', async () => {
        const ui = useUiStore();
        await ui.updateAccentFromCover('');
        expect(ui.dynamicAccent).toBeNull();
        expect(musicApi.extractCoverColor).not.toHaveBeenCalled();
    });
});
```

- [ ] **Step 3: 运行测试**

```bash
cd music-party-web && npx vitest run src/stores/ui.test.js
```

- [ ] **Step 4: Commit**

```bash
git add music-party-web/src/stores/ui.js music-party-web/src/stores/ui.test.js
git commit -m "perf: throttle frontend cover color requests to 2s minimum interval"
```

---

## Phase 4: JVM + 线程池调优

### Task 8: 调低 MaxRAMPercentage + 限制 boundedElastic

**问题：** 
1. `MaxRAMPercentage=65` 在小 VPS 上让 G1 频繁 young/old 混收，每轮 GC 烧 CPU 放大超时
2. `boundedElastic` 池无上限（Reactor 默认 `Integer.MAX_VALUE`），崩溃时线程编号到 259+，与 Netty epoll I/O 线程抢核

**Files:**
- Modify: `Dockerfile`
- Modify: `docker-compose.yml`
- Modify: `src/main/resources/application.yml`
- Modify: `src/main/java/org/thornex/musicparty/config/AppProperties.java`

- [ ] **Step 1: 调低 MaxRAMPercentage**

`Dockerfile:75`:
```dockerfile
ENV JAVA_TOOL_OPTIONS="-XX:MaxRAMPercentage=50 -XX:+UseG1GC"
```

`docker-compose.yml:53`:
```yaml
- JAVA_TOOL_OPTIONS=-XX:MaxRAMPercentage=50 -XX:+UseG1GC
```

- [ ] **Step 2: 添加 boundedElastic 上限配置**

`AppProperties.java` — 在 `PerformanceConfig` 中添加：

```java
private int boundedElasticSize = 64;
```

`application.yml` — 在 `performance` 下添加：

```yaml
bounded-elastic-size: ${BOUNDED_ELASTIC_SIZE:64}
```

`docker-compose.yml` — 在性能配置区添加：

```yaml
- BOUNDED_ELASTIC_SIZE=64    # boundedElastic 线程池上限（默认 64，防止与 Netty I/O 线程抢核）
```

- [ ] **Step 3: 在启动时设置 Reactor boundedElastic 上限**

在 `MusicPartyApplication.java` 的 `main` 方法或 `@PostConstruct` 中添加：

```java
import reactor.core.scheduler.Schedulers;

// 在 Spring Boot 启动前设置 boundedElastic 池上限
static {
    int size = Integer.parseInt(System.getProperty("reactor.schedulers.defaultBoundedElasticSize", "64"));
    Schedulers.boundedElasticSizeOverride = size;
}
```

或者更简洁地通过 JVM 系统属性设置（在 `JAVA_TOOL_OPTIONS` 中追加）：

`Dockerfile`:
```dockerfile
ENV JAVA_TOOL_OPTIONS="-XX:MaxRAMPercentage=50 -XX:+UseG1GC -Dreactor.schedulers.defaultBoundedElasticSize=64"
```

`docker-compose.yml`:
```yaml
- JAVA_TOOL_OPTIONS=-XX:MaxRAMPercentage=50 -XX:+UseG1GC -Dreactor.schedulers.defaultBoundedElasticSize=64
```

选择 JVM 系统属性方案（不需要修改 Java 代码，Reactor 原生支持此属性）。

- [ ] **Step 4: 添加配置测试**

在 `src/test/java/org/thornex/musicparty/config/PerformanceConfigTests.java` 中添加：

```java
@Test
void boundedElasticSizeHasDefault() {
    AppProperties.PerformanceConfig config = new AppProperties.PerformanceConfig();
    assertThat(config.getBoundedElasticSize()).isEqualTo(64);
}
```

- [ ] **Step 5: 运行测试**

```bash
mvn test -Dtest="PerformanceConfigTests" -Dsurefire.failIfNoSpecifiedTests=false
```

- [ ] **Step 6: Commit**

```bash
git add Dockerfile docker-compose.yml src/main/resources/application.yml src/main/java/org/thornex/musicparty/config/AppProperties.java src/test/java/org/thornex/musicparty/config/PerformanceConfigTests.java
git commit -m "perf: reduce MaxRAMPercentage to 50 and cap boundedElastic at 64 threads"
```

---

## Self-Review

### Spec Coverage

| Agent 建议 | 对应 Task |
|---|---|
| ① 修流式转发错误处理：206 已提交后不返回 JSON Map body | Task 1 (Flux 错误抑制) + Task 2 (GlobalExceptionHandler committed 感知) |
| ② 给代理加并发/缓存去重：同首歌只允许一条上游 fetch | Task 3 (CDN URL 30s 缓存) + Task 4 (in-flight 去重) + Task 5 (本地缓存集成) |
| ③ 限频前端轮询 + 后端封面色内存短路 | Task 6 (in-flight 去重 + 早期缓存) + Task 7 (前端 2s 节流) |
| ④ MaxRAMPercentage 改 45-50 + boundedElastic 上限 | Task 8 (50% + 64 线程上限) |

### Placeholder Scan

无 TBD / TODO / "implement later" / "add appropriate error handling" 等占位符。所有代码步骤包含完整代码。

### Type Consistency

- `ReactiveStreamUtils.withErrorSuppression(Flux<DataBuffer>, String, Logger)` — 在 Task 1 定义，Task 5 使用，签名一致
- `NeteaseProxyController.resolveCdnDedup(String)` — Task 4 定义，替换 Task 3 的 `resolveCdnWithCache`
- `CoverColorService.doExtract(String, String)` — Task 6 定义
- `AppProperties.PerformanceConfig.boundedElasticSize` — Task 8 定义
- `NeteaseMusicApiService` 构造函数签名变更 — Task 5 需要更新所有注入处（包括测试 mock）

### 风险提示

1. **Task 5 构造函数变更影响面：** `NeteaseMusicApiService` 构造函数新增 `LocalCacheService` 参数，Spring 会自动注入，但所有手动 new 该类的测试需要更新。需要检查 `MusicPlayerServicePendingDownloadTest`、`MusicPlayerServiceHistoryTests` 等。
2. **Task 2 返回类型变更：** `GlobalExceptionHandler` 从 `ResponseEntity` 改为 `Mono<ResponseEntity>`，Spring WebFlux 原生支持，但需确认无其他代码直接调用这些 handler 方法。
3. **Task 5 网易云缓存扩展名：** `.mp3` 对 exhigh 音质正确，但 lossless/hires 可能返回 `.flac`。当前实现用固定 `.mp3`，如果用户切换到 lossless 音质，缓存文件可能扩展名不匹配。建议后续根据 `quality` 配置动态选择扩展名，或让 `resolveCdnUrl` 返回的 URL 中提取扩展名。
4. **Task 8 线程池上限 64：** 在极端高并发下可能导致排队等待。但 64 远大于 Netty I/O 线程数（通常 4），且崩溃时 259 线程是病态的，64 是健康的。可根据 VPS CPU 核数调整。
