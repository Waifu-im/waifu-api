using System.Collections.Generic;
using System.Linq;
using System.Threading;
using System.Threading.Tasks;
using Mediator;
using Microsoft.EntityFrameworkCore;
using WaifuApi.Application.Interfaces;
using WaifuApi.Domain.Entities;

namespace WaifuApi.Application.Features.Users.GetUsers;

public record GetUsersQuery(string? Search = null, int Page = 1, int PageSize = 20) : IQuery<List<User>>;

public class GetUsersQueryHandler : IQueryHandler<GetUsersQuery, List<User>>
{
    private readonly IWaifuDbContext _context;

    public GetUsersQueryHandler(IWaifuDbContext context)
    {
        _context = context;
    }

    public async ValueTask<List<User>> Handle(GetUsersQuery request, CancellationToken cancellationToken)
    {
        var query = _context.Users.AsQueryable();

        if (!string.IsNullOrEmpty(request.Search))
        {
            query = query.Where(u => u.Name.Contains(request.Search) || u.DiscordId.Contains(request.Search));
        }

        return await query
            .OrderBy(u => u.Id)
            .Skip((request.Page - 1) * request.PageSize)
            .Take(request.PageSize)
            .ToListAsync(cancellationToken);
    }
}
