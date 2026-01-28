using System.Collections.Generic;
using System.Threading;
using System.Threading.Tasks;
using Mediator;
using WaifuApi.Application.Interfaces;
using WaifuApi.Domain.Entities;

namespace WaifuApi.Application.Features.Tags.UpdateTag;

public record UpdateTagCommand(long Id, string Name, string Description) : ICommand<Tag>;

public class UpdateTagCommandHandler : ICommandHandler<UpdateTagCommand, Tag>
{
    private readonly IWaifuDbContext _context;

    public UpdateTagCommandHandler(IWaifuDbContext context)
    {
        _context = context;
    }

    public async ValueTask<Tag> Handle(UpdateTagCommand request, CancellationToken cancellationToken)
    {
        var tag = await _context.Tags.FindAsync(new object[] { request.Id }, cancellationToken);

        if (tag == null)
        {
            throw new KeyNotFoundException($"Tag with ID {request.Id} not found.");
        }

        tag.Name = request.Name;
        tag.Description = request.Description;
        
        await _context.SaveChangesAsync(cancellationToken);

        return tag;
    }
}
