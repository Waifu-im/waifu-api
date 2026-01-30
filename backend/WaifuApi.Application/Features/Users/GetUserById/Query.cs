using System.Collections.Generic;
using System.Threading;
using System.Threading.Tasks;
using Mediator;
using WaifuApi.Application.Common.Models;
using WaifuApi.Application.Interfaces;
using WaifuApi.Domain.Entities;

namespace WaifuApi.Application.Features.Users.GetUserById;

public record GetUserByIdQuery(long UserId) : IQuery<UserMinimalDto>;

public class GetUserByIdQueryHandler : IQueryHandler<GetUserByIdQuery, UserMinimalDto>
{
    private readonly IWaifuDbContext _context;

    public GetUserByIdQueryHandler(IWaifuDbContext context)
    {
        _context = context;
    }

    public async ValueTask<UserMinimalDto> Handle(GetUserByIdQuery request, CancellationToken cancellationToken)
    {
        var user = await _context.Users.FindAsync(new object[] { request.UserId }, cancellationToken);
        if (user == null)
        {
            throw new KeyNotFoundException($"User with ID {request.UserId} not found.");
        }
        
        return new UserMinimalDto
        {
            Id = user.Id,
            Name = user.Name
        };
    }
}
