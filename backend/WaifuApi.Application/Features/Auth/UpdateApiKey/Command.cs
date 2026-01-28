using System;
using System.Collections.Generic;
using System.Threading;
using System.Threading.Tasks;
using Mediator;
using Microsoft.EntityFrameworkCore;
using WaifuApi.Application.Common.Models;
using WaifuApi.Application.Interfaces;
using WaifuApi.Domain.Entities;

namespace WaifuApi.Application.Features.Auth.UpdateApiKey;

public record UpdateApiKeyCommand(long UserId, long ApiKeyId, string Description, DateTime? ExpirationDate) : ICommand<ApiKeyDto>;

public class UpdateApiKeyCommandHandler : ICommandHandler<UpdateApiKeyCommand, ApiKeyDto>
{
    private readonly IWaifuDbContext _context;

    public UpdateApiKeyCommandHandler(IWaifuDbContext context)
    {
        _context = context;
    }

    public async ValueTask<ApiKeyDto> Handle(UpdateApiKeyCommand request, CancellationToken cancellationToken)
    {
        var apiKey = await _context.ApiKeys
            .FirstOrDefaultAsync(k => k.Id == request.ApiKeyId && k.UserId == request.UserId, cancellationToken);

        if (apiKey == null)
        {
            throw new KeyNotFoundException($"API Key with ID {request.ApiKeyId} not found.");
        }

        apiKey.Description = request.Description;
        apiKey.ExpirationDate = request.ExpirationDate;

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
}
