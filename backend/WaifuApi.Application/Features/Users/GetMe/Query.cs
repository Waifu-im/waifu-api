using System.Collections.Generic;
using System.Threading;
using System.Threading.Tasks;
using Mediator;
using WaifuApi.Application.Common.Models;
using WaifuApi.Application.Interfaces;
using WaifuApi.Domain.Entities;

namespace WaifuApi.Application.Features.Users.GetMe;

public record GetMeQuery(long UserId) : IQuery<UserDto>;

public class GetMeQueryHandler : IQueryHandler<GetMeQuery, UserDto>
{
    private readonly IWaifuDbContext _context;

    public GetMeQueryHandler(IWaifuDbContext context)
    {
        _context = context;
    }

    public async ValueTask<UserDto> Handle(GetMeQuery request, CancellationToken cancellationToken)
    {
        var user = await _context.Users.FindAsync(new object[] { request.UserId }, cancellationToken);
        if (user == null)
        {
            throw new KeyNotFoundException($"User with ID {request.UserId} not found.");
        }
        return new UserDto
        {
            Id = user.Id,
            Name = user.Name,
            DiscordId = user.DiscordId,
            AvatarUrl = user.AvatarUrl,
            Role = user.Role,
            IsBlacklisted = user.IsBlacklisted,
            RequestCount = user.RequestCount,
            ApiKeyRequestCount = user.ApiKeyRequestCount,
            JwtRequestCount = user.JwtRequestCount
        };
    }
}
