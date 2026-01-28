using System;
using System.Collections.Generic;
using System.Net;
using System.Text.Json;
using System.Threading.Tasks;
using FluentValidation;
using Microsoft.AspNetCore.Http;

namespace WaifuApi.Web.Middleware;

public class ExceptionHandlingMiddleware
{
    private readonly RequestDelegate _next;

    public ExceptionHandlingMiddleware(RequestDelegate next)
    {
        _next = next;
    }

    public async Task Invoke(HttpContext context)
    {
        try
        {
            await _next(context);
        }
        catch (Exception ex)
        {
            await HandleExceptionAsync(context, ex);
        }
    }

    private static Task HandleExceptionAsync(HttpContext context, Exception exception)
    {
        var code = HttpStatusCode.InternalServerError;
        var result = string.Empty;

        switch (exception)
        {
            case ValidationException validationException:
                code = HttpStatusCode.BadRequest;
                result = JsonSerializer.Serialize(new { errors = validationException.Errors });
                break;
            case UnauthorizedAccessException:
                code = HttpStatusCode.Unauthorized;
                break;
            case KeyNotFoundException:
                code = HttpStatusCode.NotFound;
                break;
            case ArgumentException:
                code = HttpStatusCode.BadRequest;
                result = JsonSerializer.Serialize(new { error = exception.Message });
                break;
        }

        context.Response.ContentType = "application/json";
        context.Response.StatusCode = (int)code;

        if (string.IsNullOrEmpty(result))
        {
            result = JsonSerializer.Serialize(new { error = exception.Message });
        }

        return context.Response.WriteAsync(result);
    }
}
