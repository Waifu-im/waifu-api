using System;
using System.Security.Cryptography;
using System.Text;
using System.Threading;
using System.Threading.Tasks;
using Mediator;
using WaifuApi.Application.Common.Models;
using WaifuApi.Application.Interfaces;
using WaifuApi.Domain.Entities;

namespace WaifuApi.Application.Features.Auth.CreateApiKey;

public record CreateApiKeyCommand(long UserId, string Description, DateTime? ExpirationDate) : ICommand<ApiKeyDto>;

public class CreateApiKeyCommandHandler : ICommandHandler<CreateApiKeyCommand, ApiKeyDto>
{
    private readonly IWaifuDbContext _context;

    public CreateApiKeyCommandHandler(IWaifuDbContext context)
    {
        _context = context;
    }

    public async ValueTask<ApiKeyDto> Handle(CreateApiKeyCommand request, CancellationToken cancellationToken)
    {
        var rawKey = GenerateApiKey();
        var keyHash = HashApiKey(rawKey);

        var apiKey = new ApiKey
        {
            UserId = request.UserId,
            KeyHash = keyHash,
            Description = request.Description,
            CreatedAt = DateTime.UtcNow,
            ExpirationDate = request.ExpirationDate
        };

        _context.ApiKeys.Add(apiKey);
        await _context.SaveChangesAsync(cancellationToken);

        return new ApiKeyDto
        {
            Id = apiKey.Id,
            Key = rawKey, // Only returned once at creation time
            Description = apiKey.Description,
            CreatedAt = apiKey.CreatedAt,
            LastUsedAt = apiKey.LastUsedAt,
            ExpirationDate = apiKey.ExpirationDate,
            UserId = apiKey.UserId
        };
    }

    private static string GenerateApiKey()
    {
        var bytes = new byte[32];
        using var rng = RandomNumberGenerator.Create();
        rng.GetBytes(bytes);
        return Convert.ToBase64String(bytes).Replace("+", "").Replace("/", "").Replace("=", "");
    }

    public static string HashApiKey(string rawKey)
    {
        var bytes = SHA256.HashData(Encoding.UTF8.GetBytes(rawKey));
        return Convert.ToHexStringLower(bytes);
    }
}
