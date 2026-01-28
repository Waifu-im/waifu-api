using System;
using System.Security.Cryptography;
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
        var key = GenerateApiKey();
        var apiKey = new ApiKey
        {
            UserId = request.UserId,
            Key = key,
            Description = request.Description,
            CreatedAt = DateTime.UtcNow,
            ExpirationDate = request.ExpirationDate
        };

        _context.ApiKeys.Add(apiKey);
        await _context.SaveChangesAsync(cancellationToken);

        return new ApiKeyDto
        {
            Id = apiKey.Id,
            Key = apiKey.Key,
            Description = apiKey.Description,
            CreatedAt = apiKey.CreatedAt,
            LastUsedAt = apiKey.LastUsedAt,
            ExpirationDate = apiKey.ExpirationDate,
            UserId = apiKey.UserId
        };
    }

    private string GenerateApiKey()
    {
        var bytes = new byte[32];
        using var rng = RandomNumberGenerator.Create();
        rng.GetBytes(bytes);
        return Convert.ToBase64String(bytes).Replace("+", "").Replace("/", "").Replace("=", "");
    }
}
