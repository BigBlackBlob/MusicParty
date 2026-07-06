package org.thornex.musicparty.service.api;

import org.junit.jupiter.api.Test;
import org.springframework.http.HttpStatus;
import org.springframework.web.reactive.function.client.ClientResponse;
import org.springframework.web.reactive.function.client.WebClient;
import org.thornex.musicparty.config.AppProperties;
import org.thornex.musicparty.exception.ApiRequestException;
import org.thornex.musicparty.persistence.InMemorySiteSettingRepository;
import org.thornex.musicparty.service.SiteSecretService;
import org.thornex.musicparty.service.SiteSettingService;
import org.thornex.musicparty.service.SubsonicCredentialCipher;
import reactor.core.publisher.Mono;

import static org.assertj.core.api.Assertions.assertThatThrownBy;

class NeteaseMusicApiServiceTests {

    @Test
    void resolveCdnUrlRejectsEmptyUrl() {
        NeteaseMusicApiService service = serviceReturning("""
                {"data":[{"id":1,"url":"","code":200}]}
                """);

        assertThatThrownBy(() -> service.resolveCdnUrl("1").block())
                .isInstanceOf(ApiRequestException.class)
                .hasMessageContaining("空播放地址");
    }

    @Test
    void resolveCdnUrlClassifiesExpiredCookie() {
        NeteaseMusicApiService service = serviceReturning("""
                {"data":[{"id":1,"url":null,"code":401}]}
                """);

        assertThatThrownBy(() -> service.resolveCdnUrl("1").block())
                .isInstanceOf(ApiRequestException.class)
                .hasMessageContaining("Cookie");
    }

    @Test
    void resolveCdnUrlClassifiesUnavailableCopyright() {
        NeteaseMusicApiService service = serviceReturning("""
                {"data":[{"id":1,"url":null,"code":403}]}
                """);

        assertThatThrownBy(() -> service.resolveCdnUrl("1").block())
                .isInstanceOf(ApiRequestException.class)
                .hasMessageContaining("无版权");
    }

    @Test
    void resolveCdnUrlClassifiesUpstreamErrors() {
        NeteaseMusicApiService service = serviceWithResponse(
                ClientResponse.create(HttpStatus.INTERNAL_SERVER_ERROR).body("boom").build());

        assertThatThrownBy(() -> service.resolveCdnUrl("1").block())
                .isInstanceOf(ApiRequestException.class)
                .hasMessageContaining("上游");
    }

    @Test
    void resolveCdnUrlClassifiesTimeout() {
        NeteaseMusicApiService service = serviceWithWebClient(WebClient.builder()
                .exchangeFunction(request -> Mono.never())
                .build());

        assertThatThrownBy(() -> service.resolveCdnUrl("1").block())
                .isInstanceOf(ApiRequestException.class)
                .hasMessageContaining("超时");
    }

    private NeteaseMusicApiService serviceReturning(String body) {
        return serviceWithResponse(ClientResponse.create(HttpStatus.OK)
                .header("Content-Type", "application/json")
                .body(body)
                .build());
    }

    private NeteaseMusicApiService serviceWithResponse(ClientResponse response) {
        return serviceWithWebClient(WebClient.builder()
                .exchangeFunction(request -> Mono.just(response))
                .build());
    }

    private NeteaseMusicApiService serviceWithWebClient(WebClient webClient) {
        AppProperties properties = new AppProperties();
        properties.setNetease(new AppProperties.NeteaseApiConfig());
        properties.getNetease().setBaseUrl("https://netease.test");
        properties.getNetease().setCookie("MUSIC_U=test");
        properties.getNetease().setQuality("exhigh");
        InMemorySiteSettingRepository repository = new InMemorySiteSettingRepository();
        SiteSettingService settings = new SiteSettingService(
                repository,
                new SubsonicCredentialCipher(new SiteSecretService(repository))
        );
        return new NeteaseMusicApiService(webClient, properties, settings);
    }
}
