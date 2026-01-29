using System;
using System.Collections.Generic;
using System.Net.Http;
using System.Net.Http.Json;
using System.Threading.Tasks;
using Microsoft.Extensions.Configuration;
using WaifuApi.Application.Interfaces;

namespace WaifuApi.Infrastructure.Services;

public class DiscordService : IDiscordService
{
    private readonly IHttpClientFactory _httpClientFactory;
    private readonly IConfiguration _configuration;

    public DiscordService(IHttpClientFactory httpClientFactory, IConfiguration configuration)
    {
        _httpClientFactory = httpClientFactory;
        _configuration = configuration;
    }

    public async Task<string> GetAccessTokenAsync(string code)
    {
        var client = _httpClientFactory.CreateClient();
        var response = await client.PostAsync("https://discord.com/api/oauth2/token", new FormUrlEncodedContent(new[]
        {
            new KeyValuePair<string, string>("client_id", _configuration["Discord:ClientId"]!),
            new KeyValuePair<string, string>("client_secret", _configuration["Discord:ClientSecret"]!),
            new KeyValuePair<string, string>("grant_type", "authorization_code"),
            new KeyValuePair<string, string>("code", code),
            new KeyValuePair<string, string>("redirect_uri", _configuration["Discord:RedirectUri"]!)
        }));

        if (!response.IsSuccessStatusCode)
        {
            var errorContent = await response.Content.ReadAsStringAsync();
            throw new Exception($"Discord Token Exchange Failed: {response.StatusCode} - {errorContent}");
        }

        var data = await response.Content.ReadFromJsonAsync<DiscordTokenResponse>();
        return data!.AccessToken;
    }

    public async Task<DiscordUserDto> GetUserProfileAsync(string accessToken)
    {
        var client = _httpClientFactory.CreateClient();
        var request = new HttpRequestMessage(HttpMethod.Get, "https://discord.com/api/users/@me");
        request.Headers.Authorization = new System.Net.Http.Headers.AuthenticationHeaderValue("Bearer", accessToken);

        var response = await client.SendAsync(request);
        
        if (!response.IsSuccessStatusCode)
        {
            var errorContent = await response.Content.ReadAsStringAsync();
            throw new Exception($"Discord User Profile Failed: {response.StatusCode} - {errorContent}");
        }

        var user = await response.Content.ReadFromJsonAsync<DiscordUserResponse>();
        
        return new DiscordUserDto
        {
            Id = user!.Id,
            Username = user.Username
        };
    }

    private class DiscordTokenResponse
    {
        [System.Text.Json.Serialization.JsonPropertyName("access_token")]
        public string AccessToken { get; set; } = string.Empty;
    }

    private class DiscordUserResponse
    {
        [System.Text.Json.Serialization.JsonPropertyName("id")]
        public string Id { get; set; } = string.Empty;
        [System.Text.Json.Serialization.JsonPropertyName("username")]
        public string Username { get; set; } = string.Empty;
    }
}
