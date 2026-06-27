using System;
using System.Collections.Generic;
using System.Linq;
using System.Net.Http;
using System.Net.Http.Headers;
using System.Net.Http.Json;
using System.Threading;
using System.Threading.Tasks;
using Microsoft.Extensions.Configuration;
using Microsoft.Extensions.Logging;
using WaifuApi.Application.Common.Constants;
using WaifuApi.Application.Interfaces;

namespace WaifuApi.Infrastructure.Services;

public class CloudflareCacheService : ICdnCacheService
{
    // Cloudflare allows at most 30 URLs per purge_cache call.
    private const int MaxUrlsPerPurge = 30;

    private readonly IHttpClientFactory _httpClientFactory;
    private readonly ILogger<CloudflareCacheService> _logger;
    private readonly string? _zoneId;
    private readonly string? _apiToken;
    private readonly string? _cdnBaseUrl;
    private readonly bool _transformEnabled;
    private readonly string _transformPath;
    private readonly string _widthParam;
    private readonly int[] _widths;

    public CloudflareCacheService(IConfiguration configuration, IHttpClientFactory httpClientFactory, ILogger<CloudflareCacheService> logger)
    {
        _httpClientFactory = httpClientFactory;
        _logger = logger;
        _zoneId = configuration[ConfigurationKeys.Cloudflare.ZoneId];
        _apiToken = configuration[ConfigurationKeys.Cloudflare.ApiToken];
        _cdnBaseUrl = configuration[ConfigurationKeys.Cdn.BaseUrl];

        _transformEnabled = string.Equals(configuration[ConfigurationKeys.ImageTransform.Enabled], "true", StringComparison.OrdinalIgnoreCase);
        _transformPath = configuration[ConfigurationKeys.ImageTransform.Path] ?? "";
        _widthParam = string.IsNullOrEmpty(configuration[ConfigurationKeys.ImageTransform.WidthParam]) ? "width" : configuration[ConfigurationKeys.ImageTransform.WidthParam]!;
        var card = ParseWidth(configuration[ConfigurationKeys.ImageTransform.CardWidth], 640);
        var preview = ParseWidth(configuration[ConfigurationKeys.ImageTransform.PreviewWidth], 1280);
        _widths = new[] { card, preview }.Distinct().ToArray();
    }

    private bool IsConfigured =>
        !string.IsNullOrWhiteSpace(_zoneId) && !string.IsNullOrWhiteSpace(_apiToken) && !string.IsNullOrWhiteSpace(_cdnBaseUrl);

    public Task PurgeImageAsync(long imageId, IEnumerable<string> extensions, CancellationToken ct = default)
        => PurgeFilesAsync(extensions.Where(e => !string.IsNullOrEmpty(e)).Select(ext => $"{imageId}{ext}"), ct);

    public async Task PurgeFilesAsync(IEnumerable<string> fileNames, CancellationToken ct = default)
    {
        if (!IsConfigured) return; // purging is optional; no-op when Cloudflare isn't configured

        var urls = fileNames
            .Where(f => !string.IsNullOrEmpty(f))
            .Distinct()
            .SelectMany(BuildUrls)
            .Distinct()
            .ToList();
        if (urls.Count == 0) return;

        foreach (var chunk in urls.Chunk(MaxUrlsPerPurge))
        {
            await SendPurgeAsync(chunk, ct);
        }
    }

    // Original file plus its resized variants. The original URL matches CdnUrlHelper, and the variant
    // URLs mirror the frontend cfImage helper so each purge URL matches the cached variant's key exactly.
    private IEnumerable<string> BuildUrls(string fileName)
    {
        var original = $"{_cdnBaseUrl}/{fileName}";
        yield return original;
        if (_transformEnabled && _transformPath.Length > 0)
        {
            foreach (var w in _widths)
            {
                var variant = ResizeUrl(original, w);
                if (variant != null) yield return variant;
            }
        }
    }

    private string? ResizeUrl(string originalUrl, int width)
    {
        if (!Uri.TryCreate(originalUrl, UriKind.Absolute, out var u)) return null;
        var origin = $"{u.Scheme}://{u.Authority}";
        var last = _transformPath.Length > 0 ? _transformPath[^1] : '\0';
        var sep = last == ',' || last == '/' ? "" : _transformPath.Contains('=') ? "," : "/";
        return $"{origin}{_transformPath}{sep}{_widthParam}={width}{u.AbsolutePath}";
    }

    private async Task SendPurgeAsync(IReadOnlyCollection<string> urls, CancellationToken ct)
    {
        try
        {
            var client = _httpClientFactory.CreateClient();
            using var request = new HttpRequestMessage(HttpMethod.Post,
                $"https://api.cloudflare.com/client/v4/zones/{_zoneId}/purge_cache");
            request.Headers.Authorization = new AuthenticationHeaderValue("Bearer", _apiToken);
            request.Content = JsonContent.Create(new { files = urls });

            var response = await client.SendAsync(request, ct);
            if (!response.IsSuccessStatusCode)
            {
                var body = await response.Content.ReadAsStringAsync(ct);
                _logger.LogWarning("Cloudflare cache purge failed: {Status} {Body}", (int)response.StatusCode, body);
            }
        }
        catch (Exception ex)
        {
            // Best-effort: a failed purge must not fail the delete/update. The file just stays cached until TTL.
            _logger.LogWarning(ex, "Cloudflare cache purge threw.");
        }
    }

    private static int ParseWidth(string? value, int fallback)
        => int.TryParse(value, out var w) && w > 0 ? w : fallback;
}
