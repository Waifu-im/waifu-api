using System;
using System.Security.Cryptography;
using System.Threading;
using System.Threading.Tasks;
using Mediator;
using WaifuApi.Application.Interfaces;
using WaifuApi.Domain.Entities;

namespace WaifuApi.Application.Features.Auth.CreateApiKey;

public record CreateApiKeyCommand(long UserId, string Description) : ICommand<ApiKey>;

public class CreateApiKeyCommandHandler : ICommandHandler<CreateApiKeyCommand, ApiKey>
{
    private readonly IWaifuDbContext _context;

    public CreateApiKeyCommandHandler(IWaifuDbContext context)
    {
        _context = context;
    }

    public async ValueTask<ApiKey> Handle(CreateApiKeyCommand request, CancellationToken cancellationToken)
    {
        var key = GenerateApiKey();
        var apiKey = new ApiKey
        {
            UserId = request.UserId,
            Key = key,
            Description = request.Description,
            CreatedAt = DateTime.UtcNow
        };

        _context.ApiKeys.Add(apiKey);
        await _context.SaveChangesAsync(cancellationToken);

        return apiKey;
    }

    private string GenerateApiKey()
    {
        var bytes = new byte[32];
        using var rng = RandomNumberGenerator.Create();
        rng.GetBytes(bytes);
        return Convert.ToBase64String(bytes).Replace("+", "").Replace("/", "").Replace("=", "");
    }
}
