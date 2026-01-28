using System.Collections.Generic;
using System.Threading;
using System.Threading.Tasks;
using Mediator;
using WaifuApi.Application.Interfaces;
using WaifuApi.Domain.Entities;

namespace WaifuApi.Application.Features.Users.GetMe;

public record GetMeQuery(long UserId) : IQuery<User>;

public class GetMeQueryHandler : IQueryHandler<GetMeQuery, User>
{
    private readonly IWaifuDbContext _context;

    public GetMeQueryHandler(IWaifuDbContext context)
    {
        _context = context;
    }

    public async ValueTask<User> Handle(GetMeQuery request, CancellationToken cancellationToken)
    {
        var user = await _context.Users.FindAsync(new object[] { request.UserId }, cancellationToken);
        if (user == null)
        {
            throw new KeyNotFoundException($"User with ID {request.UserId} not found.");
        }
        return user;
    }
}
