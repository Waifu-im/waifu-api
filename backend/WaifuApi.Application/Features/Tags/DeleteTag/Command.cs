using System.Collections.Generic;
using System.Threading;
using System.Threading.Tasks;
using Mediator;
using WaifuApi.Application.Interfaces;

namespace WaifuApi.Application.Features.Tags.DeleteTag;

public record DeleteTagCommand(long Id) : ICommand<Unit>;

public class DeleteTagCommandHandler : ICommandHandler<DeleteTagCommand, Unit>
{
    private readonly IWaifuDbContext _context;

    public DeleteTagCommandHandler(IWaifuDbContext context)
    {
        _context = context;
    }

    public async ValueTask<Unit> Handle(DeleteTagCommand request, CancellationToken cancellationToken)
    {
        var tag = await _context.Tags.FindAsync(new object[] { request.Id }, cancellationToken);

        if (tag == null)
        {
            throw new KeyNotFoundException($"Tag with ID {request.Id} not found.");
        }

        _context.Tags.Remove(tag);
        await _context.SaveChangesAsync(cancellationToken);

        return Unit.Value;
    }
}
