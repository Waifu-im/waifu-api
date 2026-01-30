using System.Collections.Generic;
using System.Threading;
using System.Threading.Tasks;
using Mediator;
using WaifuApi.Application.Common.Models;
using WaifuApi.Application.Interfaces;
using WaifuApi.Domain.Entities;

namespace WaifuApi.Application.Features.Tags.GetTagById;

public record GetTagByIdQuery(long TagId) : IQuery<TagDto>;

public class GetTagByIdQueryHandler : IQueryHandler<GetTagByIdQuery, TagDto>
{
    private readonly IWaifuDbContext _context;

    public GetTagByIdQueryHandler(IWaifuDbContext context)
    {
        _context = context;
    }

    public async ValueTask<TagDto> Handle(GetTagByIdQuery request, CancellationToken cancellationToken)
    {
        var tag = await _context.Tags.FindAsync(new object[] { request.TagId }, cancellationToken);
        if (tag == null)
        {
            throw new KeyNotFoundException($"Tag with ID {request.TagId} not found.");
        }
        
        return new TagDto
        {
            Id = tag.Id,
            Name = tag.Name,
            Slug = tag.Slug,
            Description = tag.Description,
            ReviewStatus = tag.ReviewStatus
        };
    }
}
