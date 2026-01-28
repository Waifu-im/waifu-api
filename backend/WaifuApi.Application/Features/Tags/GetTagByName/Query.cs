using System.Threading;
using System.Threading.Tasks;
using Mediator;
using Microsoft.EntityFrameworkCore;
using WaifuApi.Application.Common.Exceptions;
using WaifuApi.Application.Interfaces;
using WaifuApi.Domain.Entities;

namespace WaifuApi.Application.Features.Tags.GetTagByName;

public record GetTagByNameQuery(string Name) : IQuery<Tag>;

public class GetTagByNameQueryHandler : IQueryHandler<GetTagByNameQuery, Tag>
{
    private readonly IWaifuDbContext _context;

    public GetTagByNameQueryHandler(IWaifuDbContext context)
    {
        _context = context;
    }

    public async ValueTask<Tag> Handle(GetTagByNameQuery request, CancellationToken cancellationToken)
    {
        var tag = await _context.Tags
            .AsNoTracking()
            .FirstOrDefaultAsync(t => t.Name == request.Name, cancellationToken);

        if (tag == null)
        {
            throw new KeyNotFoundException($"Tag with name '{request.Name}' not found.");
        }

        return tag;
    }
}
