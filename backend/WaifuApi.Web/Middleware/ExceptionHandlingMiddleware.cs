using System;
using System.Collections.Generic;
using System.Net;
using System.Text.Json;
using System.Threading.Tasks;
using FluentValidation;
using Microsoft.AspNetCore.Http;
using Microsoft.AspNetCore.Mvc;
using WaifuApi.Application.Common.Exceptions;

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
        var statusCode = HttpStatusCode.InternalServerError;
        var problemDetails = new ProblemDetails
        {
            Title = "An error occurred while processing your request.",
            Status = (int)statusCode,
            Detail = exception.Message,
            Instance = context.Request.Path
        };

        switch (exception)
        {
            case ValidationException validationException:
                statusCode = HttpStatusCode.BadRequest;
                problemDetails.Title = "One or more validation errors occurred.";
                problemDetails.Status = (int)statusCode;
                problemDetails.Detail = "See errors property for details.";
                problemDetails.Type = "https://tools.ietf.org/html/rfc9110#section-15.5.1";
                
                var errors = new Dictionary<string, string[]>();
                foreach (var error in validationException.Errors)
                {
                    if (errors.ContainsKey(error.PropertyName))
                    {
                        var list = new List<string>(errors[error.PropertyName]);
                        list.Add(error.ErrorMessage);
                        errors[error.PropertyName] = list.ToArray();
                    }
                    else
                    {
                        errors.Add(error.PropertyName, new[] { error.ErrorMessage });
                    }
                }
                problemDetails.Extensions["errors"] = errors;
                break;
            case UnauthorizedAccessException:
                statusCode = HttpStatusCode.Unauthorized;
                problemDetails.Title = "Unauthorized";
                problemDetails.Status = (int)statusCode;
                problemDetails.Type = "https://tools.ietf.org/html/rfc9110#section-15.5.2";
                break;
            case KeyNotFoundException:
                statusCode = HttpStatusCode.NotFound;
                problemDetails.Title = "Resource Not Found";
                problemDetails.Status = (int)statusCode;
                problemDetails.Type = "https://tools.ietf.org/html/rfc9110#section-15.5.5";
                break;
            case ConflictException:
                statusCode = HttpStatusCode.Conflict;
                problemDetails.Title = "Conflict";
                problemDetails.Status = (int)statusCode;
                problemDetails.Type = "https://tools.ietf.org/html/rfc9110#section-15.5.10";
                break;
            case ArgumentException:
                statusCode = HttpStatusCode.BadRequest;
                problemDetails.Title = "Bad Request";
                problemDetails.Status = (int)statusCode;
                problemDetails.Type = "https://tools.ietf.org/html/rfc9110#section-15.5.1";
                break;
            case InvalidOperationException:
                statusCode = HttpStatusCode.BadRequest;
                problemDetails.Title = "Invalid Operation";
                problemDetails.Status = (int)statusCode;
                problemDetails.Type = "https://tools.ietf.org/html/rfc9110#section-15.5.1";
                break;
            default:
                problemDetails.Type = "https://tools.ietf.org/html/rfc9110#section-15.6.1";
                break;
        }

        problemDetails.Extensions["traceId"] = System.Diagnostics.Activity.Current?.Id ?? context.TraceIdentifier;

        context.Response.ContentType = "application/problem+json";
        context.Response.StatusCode = (int)statusCode;

        var json = JsonSerializer.Serialize(problemDetails);
        return context.Response.WriteAsync(json);
    }
}
