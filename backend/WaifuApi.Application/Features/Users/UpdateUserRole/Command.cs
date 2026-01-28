using System.Collections.Generic;
using System.Threading;
using System.Threading.Tasks;
using Mediator;
using WaifuApi.Application.Interfaces;
using WaifuApi.Domain.Entities;

namespace WaifuApi.Application.Features.Users.UpdateUserRole;

public record UpdateUserRoleCommand(long UserId, Role Role) : ICommand<User>;

public class UpdateUserRoleCommandHandler : ICommandHandler<UpdateUserRoleCommand, User>
{
    private readonly IWaifuDbContext _context;

    public UpdateUserRoleCommandHandler(IWaifuDbContext context)
    {
        _context = context;
    }

    public async ValueTask<User> Handle(UpdateUserRoleCommand request, CancellationToken cancellationToken)
    {
        var user = await _context.Users.FindAsync(new object[] { request.UserId }, cancellationToken);
        if (user == null)
        {
            throw new KeyNotFoundException($"User with ID {request.UserId} not found.");
        }

        user.Role = request.Role;
        await _context.SaveChangesAsync(cancellationToken);
        
        return user;
    }
}
